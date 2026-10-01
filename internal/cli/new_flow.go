package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/corpus"
	"github.com/iannil/jianwu/internal/engine"
	"github.com/iannil/jianwu/internal/engine/grill"
	"github.com/iannil/jianwu/internal/engine/outline"
	"github.com/iannil/jianwu/internal/engine/scaffolding"
	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/storage"
)

// checkSlugConflict returns nil if the slug is available; an error if a book exists
// and force=false. If force=true, removes existing book dir before returning nil.
// Per Q21.A3.
func checkSlugConflict(wsRoot, slug string, force bool) error {
	bookDir := filepath.Join(wsRoot, "books", slug)
	info, err := storage.OS.Stat(bookDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat %s: %w", bookDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s exists and is not a directory", bookDir)
	}
	if !force {
		return fmt.Errorf("book %q already exists at %s; use --force to overwrite", slug, bookDir)
	}
	if err := storage.OS.RemoveAll(bookDir); err != nil {
		return fmt.Errorf("remove existing book dir: %w", err)
	}
	return nil
}

// offerResume checks for incomplete sessions and asks the user whether to resume.
// Returns the session to resume (or nil to start fresh).
// Per Q11.A2.
func offerResume(repo *grill.Repository, prompt *TerminalPrompt) (*grill.Session, error) {
	incomplete, err := repo.ListIncomplete()
	if err != nil {
		return nil, fmt.Errorf("list incomplete sessions: %w", err)
	}
	if len(incomplete) == 0 {
		return nil, nil
	}
	fmt.Fprintf(prompt.Out, "\n检测到 %d 个未完成的 grill 会话:\n", len(incomplete))
	for i, s := range incomplete {
		firstTopic := s.Answers["topic"]
		if firstTopic == "" {
			firstTopic = "(未开始)"
		}
		fmt.Fprintf(prompt.Out, "  [%d] %s — %s\n", i+1, s.ID, firstTopic)
	}
	fmt.Fprintf(prompt.Out, "[回车=新会话 / 输入序号=恢复] ")
	reader := bufio.NewReader(prompt.In)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("read resume choice: %w", err)
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil
	}
	idx, err := strconv.Atoi(line)
	if err != nil || idx < 1 || idx > len(incomplete) {
		return nil, fmt.Errorf("invalid selection: %q", line)
	}
	return incomplete[idx-1], nil
}

// deriveSlugFromTopic produces a slug from the topic answer.
// Uses book.Slugify.
func deriveSlugFromTopic(topic string) string {
	return book.Slugify(topic)
}

// runNewFlow executes the full grill → outline → scaffolding pipeline.
// Returns the final outline or an error wrapped as *InfoError.
func runNewFlow(
	wsRoot string,
	cfg *config.Config,
	secrets *config.Secrets,
	prompt *TerminalPrompt,
	force bool,
	cp chatterProvider,
) (*book.Outline, error) {
	outline, _, err := runNewFlowWithChatters(wsRoot, cfg, prompt, force, cp)
	return outline, err
}

// buildChatterProvider constructs chatters for all three stages.
func buildChatterProvider(cfg *config.Config, secrets *config.Secrets) (chatterProvider, error) {
	intake, err := buildChatter(cfg, secrets, "intake")
	if err != nil {
		return chatterProvider{}, err
	}
	outline, err := buildChatter(cfg, secrets, "outline")
	if err != nil {
		return chatterProvider{}, err
	}
	scaff, err := buildChatter(cfg, secrets, "scaffolding")
	if err != nil {
		return chatterProvider{}, err
	}
	return chatterProvider{intake: intake, outline: outline, scaffolding: scaff}, nil
}

// chatterProvider bundles the three chatters needed by runNewFlow.
type chatterProvider struct {
	tracker                      *engine.TokenTracker
	intake, outline, scaffolding llm.Chatter
}

// runNewFlowWithChatters is the testable core that executes the full grill → outline → scaffolding pipeline.
// Returns the final outline and the session (archived) or an error wrapped as *InfoError.
func runNewFlowWithChatters(
	wsRoot string,
	cfg *config.Config,
	prompt *TerminalPrompt,
	force bool,
	cp chatterProvider,
) (result *book.Outline, resultSession *grill.Session, resultErr error) {
	tracker := cp.tracker
	if tracker == nil {
		tracker = &engine.TokenTracker{}
	}
	cp.intake = engine.NewTrackingChatter(cp.intake, tracker)
	cp.outline = engine.NewTrackingChatter(cp.outline, tracker)
	cp.scaffolding = engine.NewTrackingChatter(cp.scaffolding, tracker)
	tree := grill.DefaultTree()
	repo := grill.NewRepository(wsRoot)

	// 1. Resume detection
	session, err := offerResume(repo, prompt)
	if err != nil {
		return nil, nil, &InfoError{Err: err, Code: ExitCodeGeneric}
	}
	if session == nil {
		session = grill.NewSession()
	}
	// Before a book exists, keep usage with the resumable interview.
	var bookDir string
	defer func() {
		if bookDir == "" {
			if resultErr != nil {
				session.Status = grill.SessionInProgress
			}
			session.TokenUsage.Add(tracker.Snapshot())
			resultErr = errors.Join(resultErr, repo.Save(session))
			return
		}
		meta, err := book.LoadMeta(filepath.Join(bookDir, "meta.json"))
		if err == nil {
			err = persistTokenUsage(bookDir, meta, tracker)
		}
		resultErr = errors.Join(resultErr, err)
		if resultErr != nil {
			// A completed interview may still need its generation stages retried.
			session.Status = grill.SessionInProgress
			resultErr = errors.Join(resultErr, repo.Save(session))
		}
	}()

	// 2. Grill: walk tree, ask each dim
	for {
		grillCtx, grillCancel := stageCtx(cfg, "intake")
		next, err := grill.Run(grillCtx, cp.intake, tree, session, prompt)
		grillCancel()
		if err != nil {
			// Save session so user can resume.
			_ = repo.Save(session)
			return nil, session, wrapLLMError(err)
		}
		// Save after each step (resumable).
		if err := repo.Save(session); err != nil {
			return nil, session, &InfoError{Err: err, Code: ExitCodeGeneric}
		}
		if next == nil {
			break
		}
	}

	// 3. Derive slug from topic answer
	slug := deriveSlugFromTopic(session.Answers["topic"])
	if slug == "" {
		return nil, session, &InfoError{
			Err:  fmt.Errorf("could not derive slug from topic %q", session.Answers["topic"]),
			Code: ExitCodeGeneric,
		}
	}

	// Read cumulative usage before --force removes the previous book files.
	newBookDir := filepath.Join(wsRoot, "books", slug)
	var previous *book.Meta
	if force {
		previous, err = book.LoadMeta(filepath.Join(newBookDir, "meta.json"))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, session, fmt.Errorf("preserve previous token usage: %w", err)
		}
	}
	// 4. Check slug conflict
	if err := checkSlugConflict(wsRoot, slug, force); err != nil {
		return nil, session, &InfoError{Err: err, Code: ExitCodeGeneric}
	}

	// Create metadata before generation so failed outline calls remain accounted for.
	if err := writeBookMetaWithUsage(newBookDir, slug, session, previous); err != nil {
		return nil, session, &InfoError{Err: err, Code: ExitCodeGeneric}
	}
	bookDir = newBookDir
	// Keep status usable even if the outline request fails.
	if err := book.SaveOutline(filepath.Join(bookDir, "outline.json"), &book.Outline{}); err != nil {
		return nil, session, &InfoError{Err: err, Code: ExitCodeGeneric}
	}
	// 5. Outline. Workspace corpus (collected via `corpus collect`/`sync`)
	// feeds reference outlines; empty corpus degrades gracefully.
	corpusBooks, err := corpus.List(wsRoot)
	if err != nil {
		return nil, session, &InfoError{Err: fmt.Errorf("load corpus: %w", err), Code: ExitCodeGeneric}
	}
	outlineCtx, outlineCancel := stageCtx(cfg, "outline")
	outline, err := outline.Generate(outlineCtx, cp.outline, outline.Input{
		ArchetypeID: session.Answers["archetype"],
		Topic:       session.Answers["topic"],
		Audience:    session.Answers["audience"],
		Depth:       session.Answers["depth"],
		Goal:        session.Answers["goal"],
		Length:      session.Answers["length"],
		Language:    session.Answers["language"],
		CorpusBooks: corpusBooks,
	})
	outlineCancel()
	if err != nil {
		return nil, session, wrapLLMError(err)
	}

	// 6. Save book meta + outline
	if err := book.SaveOutline(filepath.Join(bookDir, "outline.json"), outline); err != nil {
		return nil, session, &InfoError{Err: err, Code: ExitCodeGeneric}
	}

	// 7. Scaffolding
	scaffCtx, scaffCancel := stageCtx(cfg, "scaffolding")
	results := scaffolding.ScaffoldAll(scaffCtx, cp.scaffolding, outline, session.Answers["archetype"],
		scaffolding.ChapterParams{
			Topic:    session.Answers["topic"],
			Audience: session.Answers["audience"],
			Depth:    session.Answers["depth"],
			Goal:     session.Answers["goal"],
			Length:   session.Answers["length"],
			Language: session.Answers["language"],
		},
		scaffolding.Options{},
	)
	scaffCancel()
	failedCount := 0
	for _, r := range results {
		if r.Err != nil {
			failedCount++
		}
	}
	if failedCount > 0 {
		fmt.Fprintf(prompt.Out, "warning: %d chapter(s) failed scaffolding; run `jianwu scaffolding <slug> --retry-failed` to retry\n", failedCount)
	}
	// Save outline with scaffolded chapters
	if err := book.SaveOutline(filepath.Join(bookDir, "outline.json"), outline); err != nil {
		return outline, session, &InfoError{Err: err, Code: ExitCodeGeneric}
	}

	// 8. Archive session to book dir as audit log
	if err := repo.Archive(session, slug); err != nil {
		// Non-fatal: log and continue.
		fmt.Fprintf(prompt.Out, "warning: could not archive session: %v\n", err)
	}

	return outline, session, nil
}

// writeBookMeta writes meta.json for the new book.
func writeBookMeta(bookDir, slug string, session *grill.Session) error {
	return writeBookMetaWithUsage(bookDir, slug, session, nil)
}

func writeBookMetaWithUsage(bookDir, slug string, session *grill.Session, previous *book.Meta) error {
	if err := storage.OS.MkdirAll(bookDir, 0o755); err != nil {
		return fmt.Errorf("mkdir book dir: %w", err)
	}
	meta := &book.Meta{
		TokenUsage:   session.TokenUsage,
		SessionUsage: map[string]book.TokenUsage{session.ID: session.TokenUsage},
		ID:           uuid.NewString(),
		Slug:         slug,
		Title:        session.Answers["topic"],
		Archetype:    session.Answers["archetype"],
		Language:     session.Answers["language"],
		Status:       book.BookStatusDraft,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		Parameters: book.Parameters{
			Audience: session.Answers["audience"],
			Depth:    session.Answers["depth"],
			Goal:     session.Answers["goal"],
			Length:   session.Answers["length"],
		},
		Engine: book.EngineMeta{
			JianwuVersion: Version,
		},
	}
	if previous != nil {
		meta.TokenUsage = previous.TokenUsage
		meta.SessionUsage = previous.SessionUsage
		if meta.SessionUsage == nil {
			meta.SessionUsage = make(map[string]book.TokenUsage)
		}
		imported := meta.SessionUsage[session.ID]
		pending := session.TokenUsage
		// Session totals are monotonic. Add only usage not already transferred.
		meta.TokenUsage.Add(book.TokenUsage{
			PromptTokens:      max(0, pending.PromptTokens-imported.PromptTokens),
			CompletionTokens:  max(0, pending.CompletionTokens-imported.CompletionTokens),
			TotalTokens:       max(0, pending.TotalTokens-imported.TotalTokens),
			CallCount:         max(0, pending.CallCount-imported.CallCount),
			CachedCount:       max(0, pending.CachedCount-imported.CachedCount),
			MissingUsageCalls: max(0, pending.MissingUsageCalls-imported.MissingUsageCalls),
		})
		meta.SessionUsage[session.ID] = session.TokenUsage
	}
	return book.SaveMeta(filepath.Join(bookDir, "meta.json"), meta)
}

// wrapLLMError classifies an error as InfoError with appropriate exit code.
func wrapLLMError(err error) error {
	if errors.Is(err, llm.ErrNetwork) {
		return &InfoError{Err: err, Code: ExitCodeNetwork}
	}
	return &InfoError{Err: err, Code: ExitCodeLLMProvider}
}

// defaultCtx returns a context.Background() with signal handling for Ctrl+C.
// The returned context is cancelled on SIGINT/SIGTERM.
func defaultCtx() context.Context {
	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	return ctx
}

// stageCtx returns a context for the given stage with:
//   - Ctrl+C cancellation (signal.NotifyContext)
//   - Per-stage timeout from config (falling back to global LLM timeout)
//   - No timeout when neither global nor stage timeout is configured (0).
//   - No timeout when cfg is nil (safe for tests).
func stageCtx(cfg *config.Config, stage string) (context.Context, context.CancelFunc) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	if cfg == nil {
		return ctx, cancel
	}
	timeout := cfg.LLM.TimeoutSeconds
	if m, err := stageModel(cfg, stage); err == nil && m.TimeoutSeconds > 0 {
		timeout = m.TimeoutSeconds
	}
	if timeout > 0 {
		timed, timeoutCancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		return timed, func() { timeoutCancel(); cancel() }
	}
	return ctx, cancel
}
