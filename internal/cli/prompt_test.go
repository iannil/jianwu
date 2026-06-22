package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/zhurong/jianwu/internal/engine/grill"
)

func TestTerminalPromptAskAcceptsEmpty(t *testing.T) {
	var out bytes.Buffer
	p := &TerminalPrompt{In: strings.NewReader("\n"), Out: &out}
	dim := grill.Dimension{
		Name:         "受众",
		Question:     "目标读者是谁？",
		Options:      []string{"scholar", "educated-general"},
		DefaultValue: "educated-general",
	}
	answer, err := p.Ask(dim, "scholar\n\nBecause topic is advanced.")
	if err != nil {
		t.Fatal(err)
	}
	if answer != "" {
		t.Errorf("expected empty (accept), got %q", answer)
	}
	s := out.String()
	if !strings.Contains(s, "◆ 受众") {
		t.Errorf("missing name header")
	}
	if !strings.Contains(s, "推荐: scholar") {
		t.Errorf("missing recommendation")
	}
}

func TestTerminalPromptAskReturnsInput(t *testing.T) {
	var out bytes.Buffer
	p := &TerminalPrompt{In: strings.NewReader("beginner\n"), Out: &out}
	dim := grill.Dimension{Name: "受众", DefaultValue: "educated-general"}
	answer, err := p.Ask(dim, "scholar")
	if err != nil {
		t.Fatal(err)
	}
	if answer != "beginner" {
		t.Errorf("got %q", answer)
	}
}

func TestTerminalPromptAskReturnsSkip(t *testing.T) {
	p := &TerminalPrompt{In: strings.NewReader("skip\n"), Out: &bytes.Buffer{}}
	answer, err := p.Ask(grill.Dimension{DefaultValue: "x"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if answer != "skip" {
		t.Errorf("got %q", answer)
	}
}

func TestTerminalPromptShowsReasoningIndented(t *testing.T) {
	var out bytes.Buffer
	p := &TerminalPrompt{In: strings.NewReader("\n"), Out: &out}
	rec := "scholar\nBecause the topic is advanced.\nIt needs deep engagement."
	_, _ = p.Ask(grill.Dimension{Name: "受众", DefaultValue: "x"}, rec)
	s := out.String()
	if !strings.Contains(s, "    Because the topic is advanced.") {
		t.Errorf("reasoning not indented")
	}
}
