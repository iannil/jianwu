package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/engine/expand"
	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/storage"
	"github.com/iannil/jianwu/internal/workspace"
)

func newExpandCmd() *cobra.Command {
	var forceCount int
	var expandAll bool
	cmd := &cobra.Command{
		Use:   "expand <slug> <NN-MM>",
		Short: "Expand one (or all) chapters into markdown with citations",
		Long: `Run the 3-iteration expand agent (research → draft → validate) on one
or all scaffolded chapters, producing chapters/NN-MM.md with YAML frontmatter
and [^N] footnote citations. Updates outline.json with status, citations,
word_count, unverified_claims.

Use --all to expand every scaffolded chapter in the book (parallel, continue-on-error).
Use --force to overwrite an existing expanded chapter.
Use --force twice (--force --force) to overwrite a reviewed or final chapter.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if expandAll {
				if len(args) != 1 {
					return &InfoError{Err: fmt.Errorf("--all requires only <slug>"), Code: ExitCodeUsage}
				}
				return runExpandAll(cmd, args[0], forceCount)
			}
			if len(args) != 2 {
				return &InfoError{Err: fmt.Errorf("requires <slug> <NN-MM> or --all"), Code: ExitCodeUsage}
			}
			return runExpand(cmd, args, forceCount, nil)
		},
	}
	cmd.Flags().CountVarP(&forceCount, "force", "f", "overwrite existing chapter (use twice to override reviewed/final)")
	cmd.Flags().BoolVar(&expandAll, "all", false, "expand all scaffolded chapters")
	return cmd
}

// runExpand is the testable core extracted from RunE.
// If deps is nil, providers are built from workspace config.
func runExpand(cmd *cobra.Command, args []string, forceCount int, deps *ProviderDeps) error {
	out := cmd.OutOrStdout()
	slug := args[0]
	addr := args[1]

	partIdx, chIdx, err := parseChapterAddr(addr)
	if err != nil {
		return &InfoError{Err: err, Code: ExitCodeUsage}
	}

	bc, err := loadBook(slug)
	if err != nil {
		return err
	}
	ws, err := workspace.Load(bc.WSRoot)
	if err != nil {
		return &InfoError{Err: err, Code: ExitCodeGeneric}
	}

	bookDir := bc.BookDir
	meta := bc.Meta
	outline := bc.Outline

	// Find the chapter.
	ch, err := findChapter(outline, partIdx, chIdx)
	if err != nil {
		return &InfoError{Err: err, Code: ExitCodeUsage}
	}

	// --force semantics (Q3=B).
	chapPath := book.ChapterPath(bookDir, partIdx, chIdx)
	if _, statErr := storage.OS.Stat(chapPath); statErr == nil {
		// File exists. Check force.
		existingFM, _, readErr := book.ReadChapter(chapPath)
		if readErr != nil {
			return &InfoError{Err: fmt.Errorf("read existing chapter: %w", readErr), Code: ExitCodeGeneric}
		}
		switch existingFM.Status {
		case book.StatusReviewed, book.StatusFinal:
			if forceCount < 2 {
				return &InfoError{
					Err:  fmt.Errorf("chapter %s has status %q; use --force --force to overwrite", addr, existingFM.Status),
					Code: ExitCodeGeneric,
				}
			}
			fmt.Fprintf(out, "warning: overwriting %s (was: %s, %d words)\n", chapPath, existingFM.Status, existingFM.WordCount)
		default:
			if forceCount < 1 {
				return &InfoError{
					Err:  fmt.Errorf("chapter %s already exists; use --force to overwrite", addr),
					Code: ExitCodeGeneric,
				}
			}
			fmt.Fprintf(out, "warning: overwriting %s (was: %s, %d words)\n", chapPath, existingFM.Status, existingFM.WordCount)
		}
	}

	// Build tool registry from provided deps.
	if deps == nil {
		secrets, err := config.LoadSecrets()
		if err != nil {
			return &InfoError{Err: fmt.Errorf("load secrets: %w", err), Code: ExitCodeLLMProvider}
		}
		deps, err = buildProviderDeps(ws.Config, secrets)
		if err != nil {
			return &InfoError{Err: err, Code: ExitCodeLLMProvider}
		}
	}
	registry, err := buildToolRegistry(deps, ws.Config, ws.Root)
	if err != nil {
		return &InfoError{Err: err, Code: ExitCodeGeneric}
	}

	// Build expand input from book meta + chapter.
	expandIn := expand.ExpandInput{
		ArchetypeID:      meta.Archetype,
		Topic:            meta.Title,
		Audience:         meta.Parameters.Audience,
		Depth:            meta.Parameters.Depth,
		Goal:             meta.Parameters.Goal,
		Length:           meta.Parameters.Length,
		Language:         meta.Language,
		PartIndex:        partIdx,
		PartTitle:        findPart(outline, partIdx).Title,
		PartRole:         findPart(outline, partIdx).Role,
		ChapterIndex:     chIdx,
		ChapterTitle:     ch.Title,
		Abstract:         ch.Abstract,
		KeyConcepts:      ch.KeyConcepts,
		WebSearchEnabled: true,
	}

	// Wire streaming if the chatter supports it and verbose mode is on.
	gf := GlobalFlagsFrom(cmd)
	if gf.Verbose {
		if streamer, ok := deps.Chatter.(llm.Streamer); ok {
			expandIn.Streamer = streamer
			expandIn.StreamOutput = os.Stdout
		}
	}

	// Adjacent chapters (same Part) for coherence; nil at Part boundaries (Q5).
	if prev, perr := findChapter(outline, partIdx, chIdx-1); perr == nil {
		expandIn.PreviousChapter = prev
	}
	if next, nerr := findChapter(outline, partIdx, chIdx+1); nerr == nil {
		expandIn.NextChapter = next
	}

	// Run expand.
	fmt.Fprintf(out, "Expanding %s/%s...\n", slug, addr)
	expandCtx, expandCancel := stageCtx(ws.Config, "expand")
	result, err := expand.Generate(expandCtx, deps.Chatter, registry, expandIn, nil)
	expandCancel()
	if err != nil {
		return wrapLLMError(err)
	}

	// Write chapter file.
	stageModel, _ := stageModel(ws.Config, "expand")
	fm := book.ChapterFrontmatter{
		Title:                 ch.Title,
		PartIndex:             partIdx,
		ChapterIndex:          chIdx,
		Status:                book.StatusExpanded,
		WordCount:             result.WordCount,
		GeneratedAt:           time.Now().UTC(),
		Model:                 stageModel.Model,
		EngineVersion:         Version,
		UnverifiedClaimsCount: len(result.UnverifiedClaims),
		Citations:             toChapterCitations(result.Citations),
	}
	if _, err := book.WriteChapter(bookDir, partIdx, chIdx, fm, result.Markdown); err != nil {
		return &InfoError{Err: err, Code: ExitCodeGeneric}
	}

	// Update outline.json.
	ch.Status = book.StatusExpanded
	ch.WordCount = result.WordCount
	ch.CitationsCount = len(result.Citations)
	ch.UnverifiedClaims = len(result.UnverifiedClaims)
	ch.Citations = toBookCitations(result.Citations)
	ch.ExpandedWith = &book.ExpandedWith{
		Provider:   stageModel.Provider,
		Model:      stageModel.Model,
		Iterations: 3,
	}

	if err := book.SaveOutline(filepath.Join(bookDir, "outline.json"), outline); err != nil {
		return &InfoError{Err: err, Code: ExitCodeGeneric}
	}

	fmt.Fprintf(out, "✓ Wrote %s\n", chapPath)
	fmt.Fprintf(out, "  Words: %d, Citations: %d, Unverified claims: %d\n",
		result.WordCount, len(result.Citations), len(result.UnverifiedClaims))
	if len(result.UnverifiedClaims) > 0 {
		fmt.Fprintf(out, "\nRun `jianwu review %s %s` after reading to approve.\n", slug, addr)
	}
	return nil
}


// toChapterCitations converts expand.Citation to book.ChapterCitation (frontmatter).
func toChapterCitations(cs []expand.Citation) []book.ChapterCitation {
	out := make([]book.ChapterCitation, 0, len(cs))
	for _, c := range cs {
		out = append(out, book.ChapterCitation{
			ID:    c.ID,
			URL:   c.URL,
			Title: c.Title,
			Site:  extractSite(c.URL),
		})
	}
	return out
}

// toBookCitations converts expand.Citation to book.Citation (outline.json).
func toBookCitations(cs []expand.Citation) []book.Citation {
	out := make([]book.Citation, 0, len(cs))
	for _, c := range cs {
		out = append(out, book.Citation{
			ID:             c.ID,
			URL:            c.URL,
			Title:          c.Title,
			AccessedAt:     c.AccessedAt,
			Snippet:        c.Snippet,
			SearchProvider: c.SearchProvider,
			ReaderProvider: c.ReaderProvider,
		})
	}
	return out
}

// extractSite returns the host portion of a URL for the frontmatter "site" field.
func extractSite(rawURL string) string {
	// Best-effort: strip scheme + path.
	s := rawURL
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	return s
}

// runExpandAll expands all scaffolded chapters in a book in parallel.
func runExpandAll(cmd *cobra.Command, slug string, forceCount int) error {
	out := cmd.OutOrStdout()

	bc, err := loadBook(slug)
	if err != nil {
		return err
	}
	ws, err := workspace.Load(bc.WSRoot)
	if err != nil {
		return &InfoError{Err: err, Code: ExitCodeGeneric}
	}
	secrets, err := config.LoadSecrets()
	if err != nil {
		return &InfoError{Err: fmt.Errorf("load secrets: %w", err), Code: ExitCodeLLMProvider}
	}
	deps, err := buildProviderDeps(ws.Config, secrets)
	if err != nil {
		return &InfoError{Err: err, Code: ExitCodeLLMProvider}
	}

	// Collect all scaffolded chapters.
	type item struct {
		partIdx, chIdx int
		ch             *book.OutlineChapter
	}
	var items []item
	for pi := range bc.Outline.Parts {
		p := &bc.Outline.Parts[pi]
		for ci := range p.Chapters {
			c := &p.Chapters[ci]
			if c.Status != book.StatusScaffolded {
				continue
			}
			items = append(items, item{partIdx: p.Index, chIdx: c.Index, ch: c})
		}
	}

	if len(items) == 0 {
		fmt.Fprintf(out, "No scaffolded chapters to expand in %s\n", slug)
		return nil
	}

	fmt.Fprintf(out, "Expanding %d scaffolded chapter(s) in %s...\n", len(items), slug)

	g, _ := errgroup.WithContext(cmd.Context())
	results := make([]struct {
		item
		err error
	}, len(items))

	for i := range items {
		i := i
		it := items[i]
		g.Go(func() error {
			addr := fmt.Sprintf("%02d-%02d", it.partIdx, it.chIdx)
			args := []string{slug, addr}
			results[i].item = it
			results[i].err = runExpand(cmd, args, forceCount, deps)
			return nil // continue-on-error
		})
	}
	_ = g.Wait() // ignore errgroup error (continue-on-error)

	// Report results.
	success, failed := 0, 0
	for _, r := range results {
		if r.err != nil {
			fmt.Fprintf(out, "  ✗ %02d-%02d: %v\n", r.partIdx, r.chIdx, r.err)
			failed++
		} else {
			success++
		}
	}
	fmt.Fprintf(out, "✓ Expand complete: %d succeeded, %d failed\n", success, failed)
	return nil
}