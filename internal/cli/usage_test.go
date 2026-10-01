package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/engine"
	"github.com/iannil/jianwu/internal/provider/llm"
)

func TestPersistTokenUsageAccumulates(t *testing.T) {
	dir := t.TempDir()
	var meta book.Meta
	if err := json.Unmarshal([]byte(`{"slug":"legacy"}`), &meta); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		tracker := &engine.TokenTracker{}
		tracker.Add(&llm.ChatResponse{Usage: llm.Usage{PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5}})
		tracker.Add(nil)
		if err := persistTokenUsage(dir, &meta, tracker); err != nil {
			t.Fatal(err)
		}
	}
	saved, err := book.LoadMeta(filepath.Join(dir, "meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	if u := saved.TokenUsage; u.CallCount != 4 || u.TotalTokens != 10 || u.MissingUsageCalls != 2 {
		t.Fatalf("usage = %+v", u)
	}
	var out bytes.Buffer
	printTokenUsage(&out, saved.TokenUsage)
	if !strings.Contains(out.String(), "Incomplete usage: 2") {
		t.Fatalf("output: %s", out.String())
	}
}
