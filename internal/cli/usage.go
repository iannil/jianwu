package cli

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/engine"
)

// persistTokenUsage merges one run exactly once; the coordinator owns meta.
func persistTokenUsage(bookDir string, meta *book.Meta, tracker *engine.TokenTracker) error {
	if tracker == nil {
		return nil
	}
	usage := tracker.Snapshot()
	if usage.CallCount == 0 {
		return nil
	}
	meta.TokenUsage.Add(usage)
	if err := book.SaveMeta(filepath.Join(bookDir, "meta.json"), meta); err != nil {
		return fmt.Errorf("save token usage: %w", err)
	}
	return nil
}
func printTokenUsage(out io.Writer, u book.TokenUsage) {
	fmt.Fprintln(out, "  LLM usage only; search and embedding costs are not included")
	fmt.Fprintf(out, "  Reported tokens: %d in + %d out = %d total (%d calls, %d cached)\n", u.PromptTokens, u.CompletionTokens, u.TotalTokens, u.CallCount, u.CachedCount)
	if u.MissingUsageCalls > 0 {
		fmt.Fprintf(out, "  Incomplete usage: %d call(s) without provider-reported tokens\n", u.MissingUsageCalls)
	}
	if u.CallCount == 0 {
		fmt.Fprintln(out, "  No recorded usage; historical consumption is unknown")
	}
}
