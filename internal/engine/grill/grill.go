package grill

import (
	"context"
	"fmt"

	"github.com/zhurong/jianwu/internal/provider/llm"
)

// UserInput is the interface for getting user answers during grill.
// In production this is a TUI/prompt; in tests it's scripted.
type UserInput interface {
	// Ask presents the question + recommendation and returns the user's answer.
	// Empty string means "accept recommendation"; "skip" means use default.
	Ask(dim Dimension, recommendation string) (string, error)
}

// Run executes one step of the grill: walk the tree to find the next dimension,
// call LLM for recommendation, ask the user, record the answer.
// Returns the updated session and the next dimension to ask (or nil if complete).
//
// Caller should call Run repeatedly until next == nil.
func Run(
	ctx context.Context,
	chatter llm.Chatter,
	tree *DesignTree,
	session *Session,
	ui UserInput,
) (next *Dimension, err error) {
	// Find the next dimension to ask.
	walk := tree.Walk(session.Answers)
	var pending *Dimension
	for i := range walk {
		d := &walk[i]
		if _, answered := session.Answers[d.ID]; answered {
			continue
		}
		pending = d
		break
	}
	if pending == nil {
		session.Status = SessionCompleted
		return nil, nil
	}

	// Get LLM recommendation.
	rec, err := Recommend(ctx, chatter, *pending, session.Answers)
	if err != nil {
		return nil, fmt.Errorf("recommend for %s: %w", pending.ID, err)
	}
	session.RecordRecommendation(pending.ID, rec)

	// Ask user.
	answer, err := ui.Ask(*pending, rec)
	if err != nil {
		return nil, fmt.Errorf("user input: %w", err)
	}

	// Resolve answer: empty = accept recommendation (first line), "skip" = default.
	final := answer
	if final == "" {
		// Use first line of recommendation.
		for i, r := range rec {
			if r == '\n' {
				final = rec[:i]
				break
			}
		}
		if final == "" {
			final = rec
		}
	} else if final == "skip" {
		final = pending.DefaultValue
	}

	session.RecordAnswer(pending.ID, final)
	session.AddTurn(Turn{
		Dimension:      pending.ID,
		Question:       pending.Question,
		Recommendation: rec,
		UserAnswer:     final,
	})
	session.CurrentDim = pending.ID

	// Return next dimension (caller iterates).
	nextWalk := tree.Walk(session.Answers)
	for i := range nextWalk {
		d := &nextWalk[i]
		if _, answered := session.Answers[d.ID]; answered {
			continue
		}
		return d, nil
	}
	session.Status = SessionCompleted
	return nil, nil
}
