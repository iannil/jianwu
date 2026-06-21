package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2EHappyPath(t *testing.T) {
	root := t.TempDir()

	// 1. init
	run := func(args ...string) (string, error) {
		cmd := NewRootCmd()
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs(args)
		// Each command resolves workspace from "." via FindWorkspace,
		// so we chdir into root for non-init commands.
		cmd.Execute()
		return out.String(), nil
	}

	// init in root
	if _, err := run("init", root); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".jianwu", "config.yaml")); err != nil {
		t.Fatalf("config.yaml not created: %v", err)
	}

	// Switch to workspace for the rest
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	// 2. info
	out, _ := run("info")
	if !strings.Contains(out, "Workspace:") {
		t.Errorf("info missing 'Workspace:': %q", out)
	}

	// 3. config set
	out, _ = run("config", "set", "models.expand.provider", "gemini")
	if !strings.Contains(out, "set models.expand.provider") {
		t.Errorf("set output unexpected: %q", out)
	}

	// 4. config get
	out, _ = run("config", "get", "models.expand.provider")
	if strings.TrimSpace(out) != "gemini" {
		t.Errorf("get after set: got %q want %q", strings.TrimSpace(out), "gemini")
	}

	// 5. config list
	out, _ = run("config", "list")
	if !strings.Contains(out, "models.expand.provider") {
		t.Errorf("list missing set key: %q", out)
	}
}
