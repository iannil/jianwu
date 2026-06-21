package cli

import (
	"bytes"
	"testing"
)

func TestRootCmdHasVersionFlag(t *testing.T) {
	cmd := NewRootCmd()
	flag := cmd.PersistentFlags().Lookup("version")
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
