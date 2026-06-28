package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/corpus"
	"github.com/iannil/jianwu/internal/provider/llmfactory"
	"github.com/iannil/jianwu/internal/storage"
	"github.com/iannil/jianwu/internal/workspace"
)

func newCorpusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "corpus",
		Short: "Manage reference corpus books",
		Long: `Manage jianwu's reference corpus — the collection of book outlines
used as inspiration and reference during book creation.

Builtin corpus is compiled into the binary. Use "corpus sync" to extend
the corpus from a zhurongshuo checkout or other JSON sources.`,
	}
	cmd.AddCommand(newCorpusListCmd())
	cmd.AddCommand(newCorpusShowCmd())
	cmd.AddCommand(newCorpusStatsCmd())
	cmd.AddCommand(newCorpusSyncCmd())
	cmd.AddCommand(newCorpusReindexCmd())
	return cmd
}

// --- list ---

func newCorpusListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all available corpus books",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCorpusList(cmd)
		},
	}
}

func runCorpusList(cmd *cobra.Command) error {
	m, ws, err := loadCorpus()
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if ws != "" {
		fmt.Fprintf(out, "Corpus books (workspace: %s):\n", ws)
	} else {
		fmt.Fprintln(out, "Corpus books (builtin):")
	}
	for _, slug := range sortedKeys(m) {
		b := m[slug]
		origin := "builtin"
		if ws != "" {
			corpusPath := filepath.Join(ws, workspace.MarkerName, workspace.CorpusDirName, slug+".json")
			if _, err := storage.OS.Stat(corpusPath); err == nil {
				origin = "workspace"
			}
		}
		fmt.Fprintf(out, "  %s  %s (%s)\n", padSlug(slug), b.Title.Zh, origin)
	}
	return nil
}

// --- show ---

func newCorpusShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <slug>",
		Short: "Show detailed info about a corpus book",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCorpusShow(cmd, args[0])
		},
	}
}

func runCorpusShow(cmd *cobra.Command, slug string) error {
	m, _, err := loadCorpus()
	if err != nil {
		return err
	}

	b, ok := m[slug]
	if !ok {
		return &InfoError{Err: fmt.Errorf("corpus book %q not found", slug), Code: ExitCodeGeneric}
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Slug:      %s\n", b.Slug)
	fmt.Fprintf(out, "Title:     %s (%s)\n", b.Title.Zh, b.Title.En)
	if b.Subtitle != "" {
		fmt.Fprintf(out, "Subtitle:  %s\n", b.Subtitle)
	}
	fmt.Fprintf(out, "Archetype: %s\n", b.Archetype)
	fmt.Fprintf(out, "Audience:  %s\n", b.Audience)
	fmt.Fprintf(out, "Depth:     %s\n", b.Depth)
	fmt.Fprintf(out, "Goal:      %s\n", b.Goal)
	fmt.Fprintf(out, "Length:    %s\n", b.Length)
	fmt.Fprintf(out, "Language:  %s\n", strings.Join(b.Language, ", "))
	fmt.Fprintf(out, "Source:    %s (%s)\n", b.Source.Name, b.Source.URL)
	fmt.Fprintf(out, "Abstract:  %s\n", b.Abstract)
	fmt.Fprintf(out, "Parts:     %d\n", len(b.Parts))
	totalCh := 0
	for _, p := range b.Parts {
		totalCh += len(p.Chapters)
		fmt.Fprintf(out, "  Part %d: %s (%d chapters)\n", p.Index, p.Title.Zh, len(p.Chapters))
	}
	fmt.Fprintf(out, "Total chapters: %d\n", totalCh)
	return nil
}

// --- stats ---

func newCorpusStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show corpus statistics",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCorpusStats(cmd)
		},
	}
}

func runCorpusStats(cmd *cobra.Command) error {
	m, _, err := loadCorpus()
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Total books:     %d\n", len(m))
	totalParts := 0
	totalChapters := 0
	archTypes := make(map[string]int)
	for _, b := range m {
		totalParts += len(b.Parts)
		for _, p := range b.Parts {
			totalChapters += len(p.Chapters)
		}
		archTypes[b.Archetype]++
	}
	fmt.Fprintf(out, "Total parts:    %d\n", totalParts)
	fmt.Fprintf(out, "Total chapters: %d\n", totalChapters)
	fmt.Fprintln(out, "Archetype distribution:")
	for _, a := range sortedKeys(archTypes) {
		fmt.Fprintf(out, "  %s: %d\n", a, archTypes[a])
	}
	return nil
}

// --- sync ---

func newCorpusSyncCmd() *cobra.Command {
	var from string
	cmd := &cobra.Command{
		Use:   "sync --from <path>",
		Short: "Sync corpus from a directory of JSON book files",
		Long: `Sync corpus books from a directory of JSON files into the workspace.

Each JSON file in <path> should be a corpus.Book in the same format as
the builtin corpus (see internal/corpus/builtin/*.json for reference).
Files are validated, then copied into .jianwu/corpus/ in the workspace,
overriding builtin books with the same slug.

Example:
  jianwu corpus sync --from ~/zhurongshuo/export/corpus/
`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if from == "" {
				return &InfoError{Err: fmt.Errorf("--from is required"), Code: ExitCodeUsage}
			}
			return runCorpusSync(cmd, from)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "directory containing corpus JSON files")
	return cmd
}

func runCorpusSync(cmd *cobra.Command, from string) error {
	wsRoot, err := workspace.FindWorkspace(findWorkspacePath())
	if err != nil {
		return &InfoError{Err: err, Code: ExitCodeWorkspaceNotFound}
	}

	// Validate source directory
	info, err := storage.OS.Stat(from)
	if err != nil {
		return &InfoError{Err: fmt.Errorf("source path: %w", err), Code: ExitCodeGeneric}
	}
	if !info.IsDir() {
		return &InfoError{Err: fmt.Errorf("%q is not a directory", from), Code: ExitCodeUsage}
	}

	// Read and validate all JSON files from source
	entries, err := storage.OS.ReadDir(from)
	if err != nil {
		return &InfoError{Err: fmt.Errorf("read source dir: %w", err), Code: ExitCodeGeneric}
	}

	var synced int
	var errors []string

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := storage.OS.ReadFile(filepath.Join(from, e.Name()))
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: read error: %v", e.Name(), err))
			continue
		}
		var b corpus.Book
		if err := json.Unmarshal(data, &b); err != nil {
			errors = append(errors, fmt.Sprintf("%s: parse error: %v", e.Name(), err))
			continue
		}
		if b.Slug == "" {
			errors = append(errors, fmt.Sprintf("%s: missing slug", e.Name()))
			continue
		}
		if b.Title.Zh == "" && b.Title.En == "" {
			errors = append(errors, fmt.Sprintf("%s: missing title", e.Name()))
			continue
		}

		// Write to workspace corpus dir
		corpusDir := filepath.Join(wsRoot, workspace.MarkerName, workspace.CorpusDirName)
		if err := storage.OS.MkdirAll(corpusDir, 0o755); err != nil {
			return &InfoError{Err: fmt.Errorf("create corpus dir: %w", err), Code: ExitCodeGeneric}
		}
		dst := filepath.Join(corpusDir, b.Slug+".json")
		if err := storage.OS.WriteFile(dst, data, 0o644); err != nil {
			errors = append(errors, fmt.Sprintf("%s: write error: %v", e.Name(), err))
			continue
		}
		synced++
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Synced %d corpus book(s) to workspace\n", synced)
	if len(errors) > 0 {
		fmt.Fprintf(out, "Errors (%d):\n", len(errors))
		for _, e := range errors {
			fmt.Fprintf(out, "  %s\n", e)
		}
	}
	return nil
}

// --- reindex ---

func newCorpusReindexCmd() *cobra.Command {
	var reindexModel string
	cmd := &cobra.Command{
		Use:   "reindex",
		Short: "Rebuild the embedding index for corpus books",
		Long: `Rebuild the embedding index file from the merged corpus
(builtin + workspace overrides). The index is used by the similar-book
tool during expand.

Uses the configured scaffolding embedder by default. Override with --model.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCorpusReindex(cmd, reindexModel)
		},
	}
	cmd.Flags().StringVar(&reindexModel, "model", "", "embedding model (default: scaffolding model from config)")
	return cmd
}

func runCorpusReindex(cmd *cobra.Command, modelOverride string) error {
	wsRoot, err := workspace.FindWorkspace(findWorkspacePath())
	if err != nil {
		return &InfoError{Err: err, Code: ExitCodeWorkspaceNotFound}
	}

	ws, err := workspace.Load(wsRoot)
	if err != nil {
		return &InfoError{Err: fmt.Errorf("load workspace: %w", err), Code: ExitCodeGeneric}
	}

	secrets, err := config.LoadSecrets()
	if err != nil {
		return &InfoError{Err: fmt.Errorf("load secrets: %w", err), Code: ExitCodeLLMProvider}
	}

	m, err := corpus.LoadWithWorkspace(wsRoot)
	if err != nil {
		return &InfoError{Err: fmt.Errorf("load corpus: %w", err), Code: ExitCodeGeneric}
	}
	if len(m) == 0 {
		return &InfoError{Err: fmt.Errorf("no corpus books to index"), Code: ExitCodeGeneric}
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Loaded %d corpus books for indexing\n", len(m))

	// Resolve the embedder model: use --model override, or scaffolding model, or fallback to first available
	modelRef := ws.Config.Models.Scaffolding
	if modelOverride != "" {
		modelRef.Model = modelOverride
	}
	if modelRef.Provider == "" {
		modelRef.Provider = "gemini"
		modelRef.Model = "gemini-2.5-flash"
	}

	embedder, err := llmfactory.NewEmbedder(modelRef, secrets)
	if err != nil {
		return &InfoError{Err: fmt.Errorf("build embedder: %w", err), Code: ExitCodeLLMProvider}
	}

	fmt.Fprintf(out, "Embedding with %s/%s...\n", modelRef.Provider, modelRef.Model)

	modelName := modelRef.Model
	if modelName == "" {
		modelName = modelRef.Provider
	}

	idx, err := corpus.BuildIndex(context.Background(), embedder, modelName, m)
	if err != nil {
		return &InfoError{Err: fmt.Errorf("build index: %w", err), Code: ExitCodeLLMProvider}
	}

	// Save to workspace
	indexPath := filepath.Join(wsRoot, workspace.MarkerName, "corpus_index.json")
	if err := corpus.SaveIndex(indexPath, idx); err != nil {
		return &InfoError{Err: fmt.Errorf("save index: %w", err), Code: ExitCodeGeneric}
	}

	fmt.Fprintf(out, "Index saved to %s (%d books, dim=%d)\n", indexPath, len(idx.Books), idx.Dim)
	return nil
}

// --- helpers ---

// loadCorpus loads corpus books, detecting workspace if available.
// Returns books, workspace root (empty if none), and error.
func loadCorpus() (map[string]*corpus.Book, string, error) {
	wsRoot, err := workspace.FindWorkspace(findWorkspacePath())
	if err == nil {
		m, err := corpus.LoadWithWorkspace(wsRoot)
		return m, wsRoot, err
	}
	// No workspace — load builtin only
	m, err := corpus.Load()
	return m, "", err
}

// sortedKeys returns sorted string keys from a map.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// padSlug right-pads a slug to a minimum width for aligned output.
func padSlug(slug string) string {
	const minWidth = 30
	if len(slug) >= minWidth {
		return slug
	}
	return slug + strings.Repeat(" ", minWidth-len(slug))
}
