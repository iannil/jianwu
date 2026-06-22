package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/zhurong/jianwu/internal/book"
	"github.com/zhurong/jianwu/internal/config"
	"github.com/zhurong/jianwu/internal/engine/expand"
	"github.com/zhurong/jianwu/internal/workspace"
)

func newExpandCmd() *cobra.Command {
	var forceCount int
	cmd := &cobra.Command{
		Use:   "expand <slug> <NN-MM>",
		Short: "Expand one chapter into markdown with citations",
		Long: `Run the 3-iteration expand agent (research → draft → validate) on one chapter,
producing chapters/NN-MM.md with YAML frontmatter and [^N] footnote citations.
Updates outline.json with status, citations, word_count, unverified_claims.

Use --force to overwrite an existing expanded chapter.
Use --force twice (--force --force) to overwrite a reviewed or final chapter.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExpand(cmd, args, forceCount)
		},
	}
	cmd.Flags().CountVarP(&forceCount, "force", "f", "overwrite existing chapter (use twice to override reviewed/final)")
	return cmd
}

// runExpand is the testable core extracted from RunE.
func runExpand(cmd *cobra.Command, args []string, forceCount int) error {
	out := cmd.OutOrStdout()
	slug := args[0]
	addr := args[1]

	partIdx, chIdx, err := parseChapterAddr(addr)
	if err != nil {
		return &InfoError{Err: err, Code: ExitCodeUsage}
	}

	wsRoot, err := workspace.FindWorkspace(".")
	if err != nil {
		return &InfoError{Err: err, Code: ExitCodeWorkspaceNotFound}
	}
	ws, err := workspace.Load(wsRoot)
	if err != nil {
		return &InfoError{Err: err, Code: ExitCodeGeneric}
	}
	secrets, _ := config.LoadSecrets()

	bookDir := filepath.Join(wsRoot, "books", slug)
	meta, err := book.LoadMeta(filepath.Join(bookDir, "meta.json"))
	if err != nil {
		return &InfoError{Err: fmt.Errorf("load meta for %q: %w", slug, err), Code: ExitCodeGeneric}
	}
	outline, err := book.LoadOutline(filepath.Join(bookDir, "outline.json"))
	if err != nil {
		return &InfoError{Err: fmt.Errorf("load outline for %q: %w", slug, err), Code: ExitCodeGeneric}
	}

	// Find the chapter.
	ch, err := findChapter(outline, partIdx, chIdx)
	if err != nil {
		return &InfoError{Err: err, Code: ExitCodeUsage}
	}

	// --force semantics (Q3=B).
	chapPath := book.ChapterPath(bookDir, partIdx, chIdx)
	if _, statErr := os.Stat(chapPath); statErr == nil {
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

	// Build providers + tool registry.
	deps, err := buildProviderDeps(ws.Config, secrets)
	if err != nil {
		return &InfoError{Err: err, Code: ExitCodeLLMProvider}
	}
	registry, err := buildToolRegistry(deps, outline)
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

	// Run expand.
	fmt.Fprintf(out, "Expanding %s/%s...\n", slug, addr)
	result, err := expand.Generate(defaultCtx(), deps.Chatter, registry, expandIn)
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
	now := time.Now().UTC()
	ch.ExpandedWith = &book.ExpandedWith{
		Provider:   stageModel.Provider,
		Model:      stageModel.Model,
		Iterations: 3,
	}
	_ = now // (ReviewedAt not set on expand; set on review)

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

// findChapter returns a pointer to the chapter at (partIdx, chIdx), or error.
// Index-based iteration so the returned pointer references outline.Parts[i].Chapters[j]
// directly (mutations persist when outline is saved).
func findChapter(outline *book.Outline, partIdx, chIdx int) (*book.OutlineChapter, error) {
	for i := range outline.Parts {
		if outline.Parts[i].Index == partIdx {
			for j := range outline.Parts[i].Chapters {
				if outline.Parts[i].Chapters[j].Index == chIdx {
					return &outline.Parts[i].Chapters[j], nil
				}
			}
			return nil, fmt.Errorf("chapter %d not found in part %d", chIdx, partIdx)
		}
	}
	return nil, fmt.Errorf("part %d not found", partIdx)
}

// findPart returns the part at partIdx, or a zero value if missing.
func findPart(outline *book.Outline, partIdx int) book.OutlinePart {
	for _, p := range outline.Parts {
		if p.Index == partIdx {
			return p
		}
	}
	return book.OutlinePart{}
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

// parseChapterAddr parses a "NN-MM" string into (partIdx, chIdx), both 1-based.
// Accepts zero-padded ("01-01") and bare ("1-1") forms.
// Returns error if format is wrong or any index is 0.
func parseChapterAddr(s string) (int, int, error) {
	if s == "" {
		return 0, 0, fmt.Errorf("empty chapter address")
	}
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid chapter address %q: want NN-MM", s)
	}
	partIdx, err := strconv.Atoi(parts[0])
	if err != nil || partIdx < 1 {
		return 0, 0, fmt.Errorf("invalid part index %q", parts[0])
	}
	chIdx, err := strconv.Atoi(parts[1])
	if err != nil || chIdx < 1 {
		return 0, 0, fmt.Errorf("invalid chapter index %q", parts[1])
	}
	return partIdx, chIdx, nil
}
