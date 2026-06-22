package grill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewSessionHasID(t *testing.T) {
	s := NewSession()
	if s.ID == "" {
		t.Error("empty ID")
	}
	if s.Status != SessionInProgress {
		t.Errorf("status: %q", s.Status)
	}
}

func TestSessionSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := NewSession()
	s.RecordAnswer("topic", "时间的实在")
	s.RecordRecommendation("topic", "From 时间的实在 to 本体论探讨")
	s.AddTurn(Turn{
		Dimension:      "topic",
		Question:       "你想写一本关于什么主题的书？",
		Recommendation: "...",
		UserAnswer:     "时间的实在",
	})
	if err := s.Save(dir); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadSession(dir, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != s.ID {
		t.Errorf("ID mismatch")
	}
	if loaded.Answers["topic"] != "时间的实在" {
		t.Errorf("answer: %q", loaded.Answers["topic"])
	}
	if loaded.Recommendations["topic"] == "" {
		t.Errorf("recommendation empty")
	}
	if len(loaded.Conversation) != 1 {
		t.Errorf("conversation turns: %d", len(loaded.Conversation))
	}
}

func TestSessionIsCompleteRequiresAllRequired(t *testing.T) {
	tree := DefaultTree()
	s := NewSession()
	if s.IsComplete(tree) {
		t.Error("empty session should not be complete")
	}
	// Fill in all required answers.
	for _, d := range tree.Dimensions {
		if !d.Required {
			continue
		}
		// Skip conditional that won't be in walk.
		s.Answers[d.ID] = "x"
	}
	if !s.IsComplete(tree) {
		t.Error("fully answered session should be complete")
	}
}

func TestSessionSaveCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "sessions")
	s := NewSession()
	if err := s.Save(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.Path(dir)); err != nil {
		t.Errorf("file not created: %v", err)
	}
}
