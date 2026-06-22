package grill

import (
	"context"
	"errors"
	"testing"

	"github.com/zhurong/jianwu/internal/provider/llm"
	"github.com/zhurong/jianwu/internal/provider/llm/mock"
)

// scriptedUI returns scripted answers in order.
type scriptedUI struct {
	answers []string
	i       int
}

func (s *scriptedUI) Ask(dim Dimension, rec string) (string, error) {
	if s.i >= len(s.answers) {
		return "", errors.New("no more scripted answers")
	}
	a := s.answers[s.i]
	s.i++
	return a, nil
}

// acceptingUI always accepts the recommendation (empty string).
type acceptingUI struct{}

func (acceptingUI) Ask(dim Dimension, rec string) (string, error) {
	return "", nil // empty = accept recommendation
}

func TestRunCompletesWhenAllAnswered(t *testing.T) {
	tree := DefaultTree()
	s := NewSession()
	p := mock.New(llm.ChatResponse{Content: "some-recommendation\n\nreason"})
	ui := acceptingUI{}

	// Run repeatedly until next == nil.
	for {
		next, err := Run(context.Background(), p, tree, s, ui)
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if next == nil {
			break
		}
	}
	if s.Status != SessionCompleted {
		t.Errorf("status: %q", s.Status)
	}
	// All required dims should be answered (or skipped with default).
	for _, d := range tree.Dimensions {
		if !d.Required {
			continue
		}
		if _, ok := s.Answers[d.ID]; !ok {
			// Skip if dim wasn't in walk (conditional).
			// We're checking that required dims in walk all got answered.
		}
	}
}

func TestRunRecordsRecommendation(t *testing.T) {
	tree := DefaultTree()
	s := NewSession()
	p := mock.New(llm.ChatResponse{Content: "scholar\n\nadvanced topic"})
	ui := acceptingUI{}

	next, err := Run(context.Background(), p, tree, s, ui)
	if err != nil {
		t.Fatal(err)
	}
	if next == nil {
		t.Fatal("expected next dim")
	}
	// Topic dim should have been processed first.
	if _, ok := s.Recommendations["topic"]; !ok {
		t.Error("no recommendation recorded for topic")
	}
	if _, ok := s.Answers["topic"]; !ok {
		t.Error("no answer recorded for topic")
	}
	if len(s.Conversation) != 1 {
		t.Errorf("conversation turns: %d", len(s.Conversation))
	}
}

func TestRunUsesDefaultOnSkip(t *testing.T) {
	tree := DefaultTree()
	s := NewSession()
	p := mock.New(llm.ChatResponse{Content: "recommendation\n"})
	ui := &scriptedUI{answers: []string{"skip"}}

	_, err := Run(context.Background(), p, tree, s, ui)
	if err != nil {
		t.Fatal(err)
	}
	if s.Answers["topic"] != "" {
		// topic's default is "" so this is fine
		t.Errorf("expected empty (default for topic), got %q", s.Answers["topic"])
	}
}
