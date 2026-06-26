package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
