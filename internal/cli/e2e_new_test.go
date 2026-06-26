package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/llm/mock"
	"github.com/iannil/jianwu/internal/workspace"
)

// TestE2ENewCommandWithMocks runs the full `jianwu new` CLI surface against
// mocked chatters injected via the testable runNewFlowWithChatters path.
// This avoids needing API keys while still exercising the cobra wiring.
func TestE2ENewCommandWithMocks(t *testing.T) {
	root := t.TempDir()
	// Initialize workspace first.
	initCmd := NewRootCmd()
	initCmd.SetArgs([]string{"init", root})
	initCmd.SetOut(&bytes.Buffer{})
	initCmd.SetErr(&bytes.Buffer{})
	if err := initCmd.Execute(); err != nil {
		t.Fatal(err)
	}

	// Load workspace config.
	ws, err := workspace.Load(root)
	if err != nil {
		t.Fatal(err)
	}

	// Set fake API keys (factory checks presence, doesn't validate).
	t.Setenv("GEMINI_API_KEY", "fake")
	t.Setenv("GLM_API_KEY", "fake")

	// Build user input: accept all recommendations (12+ empty lines).
	var inputBuf bytes.Buffer
	for i := 0; i < 15; i++ {
		inputBuf.WriteString("\n")
	}

	// Redirect stdin for TerminalPrompt.
	originalStdin := osStdin
	defer func() { osStdin = originalStdin }()
	osStdin = strings.NewReader(inputBuf.String())

	// Build mock chatters directly.
	outlineJSON := `{"parts":[{"index":1,"title":"P1","role":"ontology","chapters":[
            {"index":1,"title":"C1","status":"scaffolded"}
        ]}]}`

	scaffoldJSON := `{"abstract":"X","key_concepts":["a"],"learning_objectives":["y"],"suggested_examples":["z"]}`

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

	cp := chatterProvider{
		intake:      intakeChatter,
		outline:     mock.New(llm.ChatResponse{Content: outlineJSON}),
		scaffolding: mock.New(llm.ChatResponse{Content: scaffoldJSON}),
	}

	// Run the full new flow directly (bypass cobra).
	prompt := NewTerminalPrompt(nil, os.Stdout)
	_, _, err = runNewFlowWithChatters(root, ws.Config, prompt, false, cp)
	if err != nil {
		t.Fatalf("runNewFlowWithChatters: %v", err)
	}

	// Verify book was created.
	booksDir := filepath.Join(root, "books")
	entries, err := os.ReadDir(booksDir)
	if err != nil {
		t.Fatalf("books dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no book created")
	}
	bookDir := filepath.Join(booksDir, entries[0].Name())
	for _, want := range []string{"meta.json", "outline.json"} {
		if _, err := os.Stat(filepath.Join(bookDir, want)); err != nil {
			t.Errorf("missing %s: %v", want, err)
		}
	}
}
