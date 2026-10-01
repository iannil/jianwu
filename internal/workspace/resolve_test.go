package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/iannil/jianwu/internal/config"
)

// TestResolveRootPrecedence covers the workspace start path resolution:
// flag > JIANWU_WORKSPACE > global config `workspace:` > CWD.
func TestResolveRootPrecedence(t *testing.T) {
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.yaml")
	config.SetGlobalConfigPath(configPath)
	t.Cleanup(func() { config.SetGlobalConfigPath("") })

	if err := os.WriteFile(configPath, []byte("workspace: /from/config\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvWorkspace, "/from/env")

	cases := []struct {
		name     string
		flag     string
		wantRoot string
		wantSrc  string
	}{
		{name: "flag_wins", flag: "/from/flag", wantRoot: "/from/flag", wantSrc: SourceFlag},
		{name: "env_second", flag: "", wantRoot: "/from/env", wantSrc: SourceEnv},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root, src := ResolveRoot(c.flag)
			if root != c.wantRoot || src != c.wantSrc {
				t.Fatalf("ResolveRoot(%q) = (%q, %q), want (%q, %q)", c.flag, root, src, c.wantRoot, c.wantSrc)
			}
		})
	}

	// Env unset → config file.
	t.Setenv(EnvWorkspace, "")
	root, src := ResolveRoot("")
	if root != "/from/config" || src != SourceConfig {
		t.Fatalf("config source: got (%q, %q), want (/from/config, config)", root, src)
	}

	// Config file missing → CWD.
	if err := os.Remove(configPath); err != nil {
		t.Fatal(err)
	}
	root, src = ResolveRoot("")
	if root != "." || src != SourceCWD {
		t.Fatalf("cwd fallback: got (%q, %q), want (., cwd)", root, src)
	}

	// Invalid global config is ignored (falls through to CWD).
	if err := os.WriteFile(configPath, []byte("workspace: [broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, src = ResolveRoot("")
	if src != SourceCWD {
		t.Fatalf("invalid config should fall back to cwd, got source %q", src)
	}
}
