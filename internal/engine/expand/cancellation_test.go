package expand

import (
	"context"
	"errors"
	"testing"

	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/llm/mock"
)

type cancelClosingStreamer struct {
	cancel context.CancelFunc
	done   bool
}

func (s cancelClosingStreamer) Stream(context.Context, llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, 1)
	ch <- llm.StreamChunk{Content: "partial prose", Done: s.done}
	s.cancel()
	close(ch)
	return ch, nil
}
func TestRunDraftCanceledStreamCannotSucceed(t *testing.T) {
	for _, done := range []bool{false, true} {
		t.Run(map[bool]string{false: "closed", true: "done"}[done], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			out, err := RunDraft(ctx, mock.New(llm.ChatResponse{}), ExpandInput{Streamer: cancelClosingStreamer{cancel: cancel, done: done}}, DraftContext{}, ResearchNotes{}, nil)
			if !errors.Is(err, context.Canceled) || out != "" {
				t.Fatalf("out=%q err=%v", out, err)
			}
		})
	}
}
