package grill

import (
	"context"
	"errors"
	"testing"

	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/llm/mock"
)

func TestRecommendReturnsLLMText(t *testing.T) {
	p := mock.New(llm.ChatResponse{Content: "scholar\n\nBecause topic is advanced, scholar is the best audience."})
	dim := DefaultTree().Find("audience")
	rec, err := Recommend(context.Background(), p, *dim, map[string]string{
		"topic": "时间的实在",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec == "" {
		t.Error("empty recommendation")
	}
}

func TestRecommendPropagatesLLMError(t *testing.T) {
	p := mock.NewError(errBoom)
	dim := DefaultTree().Find("audience")
	_, err := Recommend(context.Background(), p, *dim, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

var errBoom = errors.New("boom")
