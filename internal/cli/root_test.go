package cli

import (
	"bytes"
	"os"
	"testing"

	"github.com/iannil/jianwu/internal/workspace"
)

// TestMain neutralizes ambient workspace-resolution inputs for the whole
// package. ResolveRoot precedence is flag > JIANWU_WORKSPACE > global config
// `workspace:` key > CWD; a developer shell often has the env var or global
// key set, which would break the CWD-based resolution most tests rely on.
func TestMain(m *testing.M) {
	os.Unsetenv(workspace.EnvWorkspace)
	os.Setenv("HOME", os.TempDir())
	os.Exit(m.Run())
}

func TestRootCmdHasVersionFlag(t *testing.T) {
	cmd := NewRootCmd()
	cmd.InitDefaultVersionFlag()
	flag := cmd.Flags().Lookup("version")
	if flag == nil {
		t.Error("--version flag not registered")
	}
}

func TestRootCmdVersionPrints(t *testing.T) {
	cmd := NewRootCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Len() == 0 {
		t.Error("expected version output, got nothing")
	}
}

func TestExitCodeConstants(t *testing.T) {
	if ExitCodeSuccess != 0 {
		t.Errorf("ExitCodeSuccess = %d", ExitCodeSuccess)
	}
	if ExitCodeGeneric != 1 {
		t.Errorf("ExitCodeGeneric = %d", ExitCodeGeneric)
	}
	if ExitCodeUsage != 2 {
		t.Errorf("ExitCodeUsage = %d", ExitCodeUsage)
	}
	if ExitCodeWorkspaceNotFound != 3 {
		t.Errorf("ExitCodeWorkspaceNotFound = %d", ExitCodeWorkspaceNotFound)
	}
	if ExitCodeLLMProvider != 4 {
		t.Errorf("ExitCodeLLMProvider = %d", ExitCodeLLMProvider)
	}
	if ExitCodeNetwork != 5 {
		t.Errorf("ExitCodeNetwork = %d", ExitCodeNetwork)
	}
}
