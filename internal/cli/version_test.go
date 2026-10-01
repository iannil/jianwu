package cli

import (
	"bytes"
	"fmt"
	"testing"
)

func TestVersionCommands(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   []string
	}{
		{"long flag", []string{"--version"}},
		{"short flag", []string{"-v"}},
		{"subcommand", []string{"version"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewRootCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(tt.in)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			want := fmt.Sprintf("jianwu %s (commit %s, built %s)\n", Version, Commit, BuildTime)
			if out.String() != want {
				t.Errorf("output = %q, want %q", out.String(), want)
			}
		})
	}
}

func TestVersionRejectsUnknownArguments(t *testing.T) {
	for _, in := range [][]string{{"version", "extra"}, {"version", "--bogus"}, {"unknown"}, {"--bogus"}} {
		t.Run(fmt.Sprint(in), func(t *testing.T) {
			cmd := NewRootCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(in)
			if err := cmd.Execute(); err == nil {
				t.Fatal("expected argument error")
			}
		})
	}
}
