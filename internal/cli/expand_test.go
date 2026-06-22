package cli

import (
	"bytes"
	"testing"
)

func TestParseChapterAddrValid(t *testing.T) {
	cases := []struct {
		in       string
		wantPart int
		wantCh   int
	}{
		{"01-01", 1, 1},
		{"1-1", 1, 1},
		{"12-07", 12, 7},
		{"99-99", 99, 99},
	}
	for _, c := range cases {
		gotPart, gotCh, err := parseChapterAddr(c.in)
		if err != nil {
			t.Errorf("parseChapterAddr(%q) err: %v", c.in, err)
			continue
		}
		if gotPart != c.wantPart || gotCh != c.wantCh {
			t.Errorf("parseChapterAddr(%q) = (%d, %d), want (%d, %d)",
				c.in, gotPart, gotCh, c.wantPart, c.wantCh)
		}
	}
}

func TestParseChapterAddrInvalid(t *testing.T) {
	cases := []string{
		"",
		"1",
		"1-",
		"-1",
		"1-1-1",
		"abc",
		"01-00", // chapter 0 invalid
		"00-01", // part 0 invalid
		"01-ab",
	}
	for _, c := range cases {
		_, _, err := parseChapterAddr(c)
		if err == nil {
			t.Errorf("parseChapterAddr(%q) expected error, got nil", c)
		}
	}
}

func TestExpandCmdShape(t *testing.T) {
	cmd := newExpandCmd()
	if cmd.Use != "expand <slug> <NN-MM>" {
		t.Errorf("Use = %q, want %q", cmd.Use, "expand <slug> <NN-MM>")
	}
	if cmd.Short == "" {
		t.Error("Short is empty")
	}
	// Args validation: cobra.ExactArgs(2)
	if cmd.Args == nil {
		t.Error("Args validator is nil")
	}
	// --force flag exists
	if cmd.Flags().Lookup("force") == nil {
		t.Error("--force flag missing")
	}
	// --force2 flag exists
	if cmd.Flags().Lookup("force2") == nil {
		t.Error("--force2 flag missing")
	}
}

func TestExpandCmdArgsValidation(t *testing.T) {
	cmd := newExpandCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	cases := [][]string{
		{"only-one-arg"},
		{"too", "many", "args"},
	}
	for _, args := range cases {
		err := cmd.Args(cmd, args)
		if err == nil {
			t.Errorf("expected error for args %v, got nil", args)
		}
	}

	// Valid: exactly 2 args
	if err := cmd.Args(cmd, []string{"my-book", "01-01"}); err != nil {
		t.Errorf("expected success for 2 args, got: %v", err)
	}
}
