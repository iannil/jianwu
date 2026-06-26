package grill

import (
	"context"
	"fmt"

	"github.com/iannil/jianwu/internal/provider/llm"
)

// UserInput is the interface for getting user answers during grill.
// In production this is a TUI/prompt; in tests it's scripted.
type UserInput interface {
	// Ask presents the question + recommendation and returns the user's answer.
	// Empty string means "accept recommendation"; "skip" means use default.
	Ask(dim Dimension, recommendation string) (string, error)
}

// extractFirstLine returns the first line of s (up to the first '\n').
// If s contains no newline, the entire string is returned.
// Used to parse the recommended value from LLM output (first line = value, rest = reasoning).
func extractFirstLine(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}
	return s
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
	pending := tree.NextPending(session.Answers)
	if pending == nil {
		session.Status = SessionCompleted
		return nil, nil
	}

	// Check context before potentially expensive LLM call.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Get LLM recommendation.
	rec, err := Recommend(ctx, chatter, *pending, session.Answers)
	if err != nil {
		return nil, fmt.Errorf("recommend for %s: %w", pending.ID, err)
	}
	session.RecordRecommendation(pending.ID, rec)

	// Check context before potentially blocking user input.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Ask user.
	answer, err := ui.Ask(*pending, rec)
	if err != nil {
		return nil, fmt.Errorf("user input: %w", err)
	}

	// Resolve answer: empty = accept recommendation (first line), "skip" = default.
	final := answer
	if final == "" {
		// First line of recommendation is the recommended value; rest is reasoning.
		final = extractFirstLine(rec)
	} else if final == "skip" {
		final = pending.DefaultValue
	}

	// Validate answer against dimension options. If invalid and a default exists,
	// fall back to the default to avoid propagating bad values downstream.
	if !pending.ValidateAnswer(final) && pending.DefaultValue != "" {
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
	if next := tree.NextPending(session.Answers); next != nil {
		return next, nil
	}
	session.Status = SessionCompleted
	return nil, nil
}
