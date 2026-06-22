package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/zhurong/jianwu/internal/book"
	"github.com/zhurong/jianwu/internal/config"
	"github.com/zhurong/jianwu/internal/engine/grill"
	"github.com/zhurong/jianwu/internal/engine/outline"
	"github.com/zhurong/jianwu/internal/engine/scaffolding"
	"github.com/zhurong/jianwu/internal/provider/llm"
)

// checkSlugConflict returns nil if the slug is available; an error if a book exists
// and force=false. If force=true, removes existing book dir before returning nil.
// Per Q21.A3.
func checkSlugConflict(wsRoot, slug string, force bool) error {
	bookDir := filepath.Join(wsRoot, "books", slug)
	info, err := os.Stat(bookDir)
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
	if err := os.RemoveAll(bookDir); err != nil {
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

// chatterProviderHook allows tests to inject mock chatters without going through
// the real factory. Production code uses the real factory via buildChatterProvider.
var chatterProviderHook = func(cfg *config.Config, secrets *config.Secrets) (chatterProvider, error) {
	return buildChatterProvider(cfg, secrets)
}

// runNewFlow executes the full grill → outline → scaffolding pipeline.
// Returns the final outline and the session (archived) or an error wrapped as *InfoError.
// This is the public version that builds chatters from config then delegates.
func runNewFlow(
	wsRoot string,
	cfg *config.Config,
	secrets *config.Secrets,
	prompt *TerminalPrompt,
	force bool,
) (*book.Outline, *grill.Session, error) {
	cp, err := chatterProviderHook(cfg, secrets)
	if err != nil {
		return nil, nil, &InfoError{Err: err, Code: ExitCodeLLMProvider}
	}
	return runNewFlowWithChatters(wsRoot, prompt, force, cp)
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
	intake, outline, scaffolding llm.Chatter
}

// runNewFlowWithChatters is the testable core that executes the full grill → outline → scaffolding pipeline.
// Returns the final outline and the session (archived) or an error wrapped as *InfoError.
func runNewFlowWithChatters(
	wsRoot string,
	prompt *TerminalPrompt,
	force bool,
	cp chatterProvider,
) (*book.Outline, *grill.Session, error) {
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

	// 2. Grill: walk tree, ask each dim
	for {
		next, err := grill.Run(defaultCtx(), cp.intake, tree, session, prompt)
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

	// 4. Check slug conflict
	if err := checkSlugConflict(wsRoot, slug, force); err != nil {
		return nil, session, &InfoError{Err: err, Code: ExitCodeGeneric}
	}

	// 5. Outline
	outline, err := outline.Generate(defaultCtx(), cp.outline, outline.Input{
		ArchetypeID: session.Answers["archetype"],
		Topic:       session.Answers["topic"],
		Audience:    session.Answers["audience"],
		Depth:       session.Answers["depth"],
		Goal:        session.Answers["goal"],
		Length:      session.Answers["length"],
		Language:    session.Answers["language"],
	})
	if err != nil {
		return nil, session, wrapLLMError(err)
	}

	// 6. Save book meta + outline
	bookDir := filepath.Join(wsRoot, "books", slug)
	cfg := &config.Config{} // Minimal config for writeBookMeta
	if err := writeBookMeta(bookDir, slug, session, cfg); err != nil {
		return nil, session, &InfoError{Err: err, Code: ExitCodeGeneric}
	}
	if err := book.SaveOutline(filepath.Join(bookDir, "outline.json"), outline); err != nil {
		return nil, session, &InfoError{Err: err, Code: ExitCodeGeneric}
	}

	// 7. Scaffolding
	scaffolding.ScaffoldAll(defaultCtx(), cp.scaffolding, outline, session.Answers["archetype"],
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
func writeBookMeta(bookDir, slug string, session *grill.Session, cfg *config.Config) error {
	if err := os.MkdirAll(bookDir, 0o755); err != nil {
		return fmt.Errorf("mkdir book dir: %w", err)
	}
	meta := &book.Meta{
		ID:        uuid.NewString(),
		Slug:      slug,
		Title:     session.Answers["topic"],
		Archetype: session.Answers["archetype"],
		Language:  session.Answers["language"],
		Status:    book.BookStatusDraft,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Parameters: book.Parameters{
			Audience: session.Answers["audience"],
			Depth:    session.Answers["depth"],
			Goal:     session.Answers["goal"],
			Length:   session.Answers["length"],
		},
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

// defaultCtx returns a context with no timeout (S6 doesn't add timeouts; user can Ctrl+C).
func defaultCtx() context.Context {
	return context.Background()
}
