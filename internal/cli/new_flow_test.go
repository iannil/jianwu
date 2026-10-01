package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/engine/grill"
	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/llm/mock"
	"github.com/iannil/jianwu/internal/workspace"
)

// countingChatter returns different responses on each call
type countingChatter struct {
	responses []llm.ChatResponse
	calls     int
}

func (c *countingChatter) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if c.calls < len(c.responses) {
		resp := c.responses[c.calls]
		c.calls++
		return &resp, nil
	}
	return &llm.ChatResponse{Content: "fallback\nreason"}, nil
}

func TestCheckSlugConflictEmpty(t *testing.T) {
	ws := t.TempDir()
	if err := checkSlugConflict(ws, "my-book", false); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestCheckSlugConflictExistingNoForce(t *testing.T) {
	ws := t.TempDir()
	bookDir := filepath.Join(ws, "books", "my-book")
	if err := os.MkdirAll(bookDir, 0o755); err != nil {
		t.Fatal(err)
	}
	err := checkSlugConflict(ws, "my-book", false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error: %v", err)
	}
}

func TestCheckSlugConflictExistingForceRemoves(t *testing.T) {
	ws := t.TempDir()
	bookDir := filepath.Join(ws, "books", "my-book")
	if err := os.MkdirAll(filepath.Join(bookDir, "chapters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bookDir, "meta.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkSlugConflict(ws, "my-book", true); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if _, err := os.Stat(bookDir); !os.IsNotExist(err) {
		t.Errorf("book dir should be removed")
	}
}

func TestOfferResumeNoSessions(t *testing.T) {
	ws := t.TempDir()
	repo := grill.NewRepository(ws)
	var out bytes.Buffer
	p := &TerminalPrompt{In: strings.NewReader(""), Out: &out}
	s, err := offerResume(repo, p)
	if err != nil {
		t.Fatal(err)
	}
	if s != nil {
		t.Errorf("expected nil, got %v", s)
	}
}

func TestOfferResumeWithChoice(t *testing.T) {
	ws := t.TempDir()
	repo := grill.NewRepository(ws)
	s := grill.NewSession()
	s.RecordAnswer("topic", "时间的实在")
	if err := repo.Save(s); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	p := &TerminalPrompt{In: strings.NewReader("1\n"), Out: &out}
	loaded, err := offerResume(repo, p)
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil || loaded.ID != s.ID {
		t.Errorf("expected resumed session %s, got %v", s.ID, loaded)
	}
}

func TestOfferResumeEmptyInputStartsFresh(t *testing.T) {
	ws := t.TempDir()
	repo := grill.NewRepository(ws)
	s := grill.NewSession()
	s.RecordAnswer("topic", "X")
	if err := repo.Save(s); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	p := &TerminalPrompt{In: strings.NewReader("\n"), Out: &out}
	loaded, err := offerResume(repo, p)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != nil {
		t.Errorf("expected nil (fresh start), got %v", loaded)
	}
}

func TestDeriveSlugFromTopic(t *testing.T) {
	s := deriveSlugFromTopic("Reality of Time")
	if s != "reality-of-time" {
		t.Errorf("got %q", s)
	}
}

// TestRunNewFlowWithChattersHappyPath tests the full orchestrator with mock chatters.
func TestRunNewFlowWithChattersHappyPath(t *testing.T) {
	ws := t.TempDir()
	// Init workspace.
	if err := workspace.Init(ws, workspace.InitOpts{}); err != nil {
		t.Fatal(err)
	}

	// Create mock chatters
	outlineJSON := `{"parts":[{"index":1,"title":"P1","role":"ontology","chapters":[
		{"index":1,"title":"C1","status":"scaffolded"}
	]}]}`
	outlineChatter := mock.New(llm.ChatResponse{Content: outlineJSON})

	scaffoldJSON := `{"abstract":"X","key_concepts":["a"],"learning_objectives":["y"],"suggested_examples":["z"]}`
	scaffChatter := mock.New(llm.ChatResponse{Content: scaffoldJSON})

	// Intake: scripted recommendations - return different values per dimension
	// Return proper values for each dimension in order: topic, audience, goal, archetype, depth, length, language, scope, example_type, visualization, timeliness, citation_style
	intakeChatter := &countingChatter{
		responses: []llm.ChatResponse{
			{Content: "Time Reality\nThe nature of time"},                        // topic
			{Content: "scholar\nAcademic researchers"},                           // audience
			{Content: "understanding\nDeep comprehension"},                       // goal
			{Content: "ontology-epistemology-practice\nPhilosophical structure"}, // archetype
			{Content: "advanced\nExpert level"},                                  // depth
			{Content: "medium\nStandard length"},                                 // length
			{Content: "zh\nChinese language"},                                    // language
			{Content: "single\nSingle volume"},                                   // scope
			{Content: "case\nCase studies"},                                      // example_type
			{Content: "tables\nTables and charts"},                               // visualization
			{Content: "timeless\nEternal relevance"},                             // timeliness
			{Content: "academic\nAcademic citations"},                            // citation_style (triggered by scholar)
		},
	}

	// User input: accept all recommendations (empty lines) for each dim
	// Default tree has 12 dims. We need one empty line per dimension asked.
	inputLines := []string{}
	for i := 0; i < 15; i++ { // generous
		inputLines = append(inputLines, "")
	}
	userInput := strings.Join(inputLines, "\n") + "\n"

	prompt := &TerminalPrompt{
		In:  strings.NewReader(userInput),
		Out: &bytes.Buffer{},
	}

	cp := chatterProvider{
		intake:      intakeChatter,
		outline:     outlineChatter,
		scaffolding: scaffChatter,
	}

	outline, session, err := runNewFlowWithChatters(ws, &config.Config{}, prompt, false, cp)
	if err != nil {
		t.Fatalf("runNewFlowWithChatters: %v", err)
	}
	if outline == nil {
		t.Fatal("nil outline")
	}
	if session == nil {
		t.Fatal("nil session")
	}
	if session.Status != grill.SessionCompleted {
		t.Errorf("expected session completed, got %v", session.Status)
	}

	// Book dir should exist with meta.json and outline.json
	entries, err := os.ReadDir(filepath.Join(ws, "books"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no book created")
	}

	// Find the book directory (should be exactly one)
	bookSlug := entries[0].Name()
	bookDir := filepath.Join(ws, "books", bookSlug)

	// Check meta.json exists
	if _, err := os.Stat(filepath.Join(bookDir, "meta.json")); os.IsNotExist(err) {
		t.Errorf("meta.json not created in %s", bookDir)
	}

	meta, err := book.LoadMeta(filepath.Join(bookDir, "meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	if meta.TokenUsage.CallCount < 3 || meta.TokenUsage.MissingUsageCalls != meta.TokenUsage.CallCount {
		t.Fatalf("all three stages must record unreported calls: %+v", meta.TokenUsage)
	}

	// Check outline.json exists
	if _, err := os.Stat(filepath.Join(bookDir, "outline.json")); os.IsNotExist(err) {
		t.Errorf("outline.json not created in %s", bookDir)
	}

	// Check session was archived
	if _, err := os.Stat(filepath.Join(bookDir, ".session.json")); os.IsNotExist(err) {
		t.Errorf("session not archived to %s", bookDir)
	}

	// Verify session was removed from active sessions
	sessionsDir := filepath.Join(ws, ".jianwu", "sessions")
	activeEntries, err := os.ReadDir(sessionsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(activeEntries) != 0 {
		t.Errorf("expected no active sessions, found %d", len(activeEntries))
	}
}

type failedUsageChatter struct{}

func (failedUsageChatter) Chat(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Usage: llm.Usage{TotalTokens: 11}}, llm.ErrLLMProvider
}
func TestNewIntakeFailurePreservesUsageInSession(t *testing.T) {
	ws := t.TempDir()
	if err := workspace.Init(ws, workspace.InitOpts{}); err != nil {
		t.Fatal(err)
	}
	prompt := &TerminalPrompt{In: strings.NewReader(""), Out: &bytes.Buffer{}}
	_, session, err := runNewFlowWithChatters(ws, &config.Config{}, prompt, false, chatterProvider{intake: failedUsageChatter{}})
	if err == nil || session == nil {
		t.Fatalf("session=%v err=%v", session, err)
	}
	pending, err := grill.NewRepository(ws).ListIncomplete()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].TokenUsage.TotalTokens != 11 {
		t.Fatalf("pending usage: %+v", pending)
	}
}

func TestNewOutlineFailurePersistsBookUsage(t *testing.T) {
	ws := t.TempDir()
	if err := workspace.Init(ws, workspace.InitOpts{}); err != nil {
		t.Fatal(err)
	}
	session := grill.NewSession()
	session.Answers = map[string]string{"topic": "usage-failure", "audience": "scholar", "goal": "understanding", "archetype": "ontology-epistemology-practice", "depth": "advanced", "length": "medium", "language": "zh", "scope": "single", "example_type": "case", "visualization": "tables", "timeliness": "timeless", "citation_style": "academic"}
	session.TokenUsage.TotalTokens = 5
	session.TokenUsage.CallCount = 1
	if err := grill.NewRepository(ws).Save(session); err != nil {
		t.Fatal(err)
	}
	_, _, err := runNewFlowWithChatters(ws, &config.Config{}, &TerminalPrompt{In: strings.NewReader("1\n"), Out: &bytes.Buffer{}}, false, chatterProvider{intake: failedUsageChatter{}, outline: failedUsageChatter{}})
	if err == nil {
		t.Fatal("expected outline failure")
	}
	dir := filepath.Join(ws, "books", "usage-failure")
	meta, err := book.LoadMeta(filepath.Join(dir, "meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	if meta.TokenUsage.TotalTokens != 16 || meta.TokenUsage.CallCount != 2 {
		t.Fatalf("usage = %+v", meta.TokenUsage)
	}
	if _, err := book.LoadOutline(filepath.Join(dir, "outline.json")); err != nil {
		t.Fatal(err)
	}
}

func TestNewRepeatedOutlineFailuresAccumulate(t *testing.T) {
	ws := t.TempDir()
	if err := workspace.Init(ws, workspace.InitOpts{}); err != nil {
		t.Fatal(err)
	}
	session := grill.NewSession()
	session.Answers = map[string]string{"topic": "retry-usage", "audience": "scholar", "goal": "understanding", "archetype": "ontology-epistemology-practice", "depth": "advanced", "length": "medium", "language": "zh", "scope": "single", "example_type": "case", "visualization": "tables", "timeliness": "timeless", "citation_style": "academic"}
	session.TokenUsage = book.TokenUsage{TotalTokens: 5, CallCount: 1}
	repo := grill.NewRepository(ws)
	if err := repo.Save(session); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		_, _, err := runNewFlowWithChatters(ws, &config.Config{}, &TerminalPrompt{In: strings.NewReader("1\n"), Out: &bytes.Buffer{}}, i > 0, chatterProvider{intake: failedUsageChatter{}, outline: failedUsageChatter{}})
		if err == nil {
			t.Fatal("expected outline failure")
		}
		meta, err := book.LoadMeta(filepath.Join(ws, "books", "retry-usage", "meta.json"))
		if err != nil {
			t.Fatal(err)
		}
		if meta.TokenUsage.TotalTokens != 5+11*(i+1) || meta.TokenUsage.CallCount != i+2 {
			t.Fatalf("attempt %d usage=%+v", i, meta.TokenUsage)
		}
	}
}

func TestForceNewUsageSessionCheckpoints(t *testing.T) {
	dir := t.TempDir()
	previous := &book.Meta{TokenUsage: book.TokenUsage{TotalTokens: 100, CallCount: 10}, SessionUsage: map[string]book.TokenUsage{"first": {TotalTokens: 5, CallCount: 1}}}
	for _, tc := range []struct {
		name, id              string
		pending               book.TokenUsage
		wantTokens, wantCalls int
	}{
		{name: "same session imported once", id: "first", pending: book.TokenUsage{TotalTokens: 5, CallCount: 1}, wantTokens: 100, wantCalls: 10},
		{name: "new session independent usage", id: "second", pending: book.TokenUsage{TotalTokens: 7, CallCount: 1}, wantTokens: 107, wantCalls: 11},
		{name: "first session gains pending usage", id: "first", pending: book.TokenUsage{TotalTokens: 8, CallCount: 2}, wantTokens: 110, wantCalls: 12},
		{name: "second session resumes again", id: "second", pending: book.TokenUsage{TotalTokens: 7, CallCount: 1}, wantTokens: 110, wantCalls: 12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			session := grill.NewSession()
			session.ID = tc.id
			session.TokenUsage = tc.pending
			if err := writeBookMetaWithUsage(dir, "retry", session, previous); err != nil {
				t.Fatal(err)
			}
			saved, err := book.LoadMeta(filepath.Join(dir, "meta.json"))
			if err != nil {
				t.Fatal(err)
			}
			if saved.TokenUsage.TotalTokens != tc.wantTokens || saved.TokenUsage.CallCount != tc.wantCalls {
				t.Fatalf("usage=%+v", saved.TokenUsage)
			}
			previous = saved
		})
	}
}
