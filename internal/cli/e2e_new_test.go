package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/llm/mock"
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

	// Switch into the workspace.
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(root); err != nil {
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

	// Inject mock chatters by monkey-patching buildChatterProvider.
	// For testability we need to expose a hook. Add a package-level var:
	//   var chatterProviderForTest = buildChatterProvider  (production default)
	// Tests can override.
	originalProvider := chatterProviderHook
	defer func() { chatterProviderHook = originalProvider }()

	outlineJSON := `{"parts":[{"index":1,"title":"P1","role":"ontology","chapters":[
            {"index":1,"title":"C1","status":"scaffolded"}
        ]}]}`

	scaffoldJSON := `{"abstract":"X","key_concepts":["a"],"learning_objectives":["y"],"suggested_examples":["z"]}`

	// Intake: scripted recommendations - return different values per dimension
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

	chatterProviderHook = func(_ *config.Config, _ *config.Secrets) (chatterProvider, error) {
		return chatterProvider{
			intake:      intakeChatter,
			outline:     mock.New(llm.ChatResponse{Content: outlineJSON}),
			scaffolding: mock.New(llm.ChatResponse{Content: scaffoldJSON}),
		}, nil
	}

	// The cobra command's stdin needs to be set, but TerminalPrompt uses os.Stdin directly.
	// For testing, we redirect osStdin (which is a package var).
	originalStdin := osStdin
	defer func() { osStdin = originalStdin }()
	osStdin = strings.NewReader(inputBuf.String())

	cmd := NewRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"new"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("new command: %v", err)
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
