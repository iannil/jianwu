package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/iannil/jianwu/internal/engine/grill"
)

var (
	osStdin  io.Reader = os.Stdin
	osStdout io.Writer = os.Stdout
)

// TerminalPrompt implements grill.UserInput via bufio.Scanner over stdin/stdout.
type TerminalPrompt struct {
	In  io.Reader
	Out io.Writer
}

// NewTerminalPrompt constructs a TerminalPrompt using the given reader/writer.
// Defaults to os.Stdin / os.Stdout if nil.
func NewTerminalPrompt(in io.Reader, out io.Writer) *TerminalPrompt {
	if in == nil {
		in = stdin()
	}
	if out == nil {
		out = stdout()
	}
	return &TerminalPrompt{In: in, Out: out}
}

// Ask presents the question + recommendation, returns user's answer.
// Empty input = accept recommendation; "skip" = use default.
// Multiline recommendations are shown indented under the question.
func (p *TerminalPrompt) Ask(dim grill.Dimension, recommendation string) (string, error) {
	fmt.Fprintf(p.Out, "\n◆ %s\n", dim.Name)
	fmt.Fprintf(p.Out, "  %s\n", dim.Question)
	if len(dim.Options) > 0 {
		fmt.Fprintf(p.Out, "  选项: %s\n", strings.Join(dim.Options, ", "))
	}
	if recommendation != "" {
		firstLine := recommendation
		if i := strings.IndexByte(recommendation, '\n'); i >= 0 {
			firstLine = recommendation[:i]
		}
		fmt.Fprintf(p.Out, "  推荐: %s\n", firstLine)
		// If there's reasoning, show it indented.
		if i := strings.IndexByte(recommendation, '\n'); i >= 0 {
			reasoning := strings.TrimSpace(recommendation[i+1:])
			if reasoning != "" {
				for _, line := range strings.Split(reasoning, "\n") {
					fmt.Fprintf(p.Out, "    %s\n", line)
				}
			}
		}
	}
	fmt.Fprintf(p.Out, "  [回车=接受推荐 / 输入值 / skip=默认(%s)] ", dim.DefaultValue)
	reader := bufio.NewReader(p.In)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read input: %w", err)
	}
	answer := strings.TrimSpace(line)
	return answer, nil
}

// stdin/stdout indirection lets tests inject readers/writers.
// Real implementations just return os.Stdin / os.Stdout.
func stdin() io.Reader  { return osStdin }
func stdout() io.Writer { return osStdout }
