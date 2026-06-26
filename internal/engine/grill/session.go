package grill

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/iannil/jianwu/internal/storage"
)

// Session is the persisted state of a grill interview.
type Session struct {
	ID              string            `json:"id"`
	StartedAt       time.Time         `json:"started_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	Status          SessionStatus     `json:"status"`
	CurrentDim      string            `json:"current_dimension,omitempty"`
	Answers         map[string]string `json:"answers"`
	Recommendations map[string]string `json:"recommendations,omitempty"`
	Conversation    []Turn            `json:"conversation,omitempty"`
}

// SessionStatus: in_progress / completed / abandoned.
type SessionStatus string

const (
	SessionInProgress SessionStatus = "in_progress"
	SessionCompleted  SessionStatus = "completed"
	SessionAbandoned  SessionStatus = "abandoned"
)

// Turn is one round of grill conversation.
type Turn struct {
	Dimension      string    `json:"dimension"`
	Question       string    `json:"question,omitempty"`
	Recommendation string    `json:"recommendation,omitempty"`
	UserAnswer     string    `json:"user_answer,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
}

// NewSession creates a fresh session with a new ID.
func NewSession() *Session {
	now := time.Now().UTC()
	return &Session{
		ID:              generateSessionID(),
		StartedAt:       now,
		UpdatedAt:       now,
		Status:          SessionInProgress,
		Answers:         map[string]string{},
		Recommendations: map[string]string{},
	}
}

// RecordAnswer stores the user's answer for a dimension and bumps UpdatedAt.
func (s *Session) RecordAnswer(dimID, answer string) {
	if s.Answers == nil {
		s.Answers = map[string]string{}
	}
	s.Answers[dimID] = answer
	s.UpdatedAt = time.Now().UTC()
}

// RecordRecommendation stores the LLM-generated recommendation for a dimension.
func (s *Session) RecordRecommendation(dimID, rec string) {
	if s.Recommendations == nil {
		s.Recommendations = map[string]string{}
	}
	s.Recommendations[dimID] = rec
	s.UpdatedAt = time.Now().UTC()
}

// AddTurn appends a conversation turn.
func (s *Session) AddTurn(turn Turn) {
	if turn.Timestamp.IsZero() {
		turn.Timestamp = time.Now().UTC()
	}
	s.Conversation = append(s.Conversation, turn)
	s.UpdatedAt = time.Now().UTC()
}

// IsComplete reports whether all required dimensions are answered.
// Conditional dimensions not in the walk are skipped.
func (s *Session) IsComplete(tree *DesignTree) bool {
	walk := tree.Walk(s.Answers)
	for _, d := range walk {
		if !d.Required {
			continue
		}
		if _, ok := s.Answers[d.ID]; !ok {
			return false
		}
	}
	return true
}

// Path returns the file path for this session in the given sessions directory.
func (s *Session) Path(dir string) string {
	return filepath.Join(dir, s.ID+".json")
}

// Save writes the session to disk as pretty JSON.
func (s *Session) Save(dir string) error {
	if err := storage.OS.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir sessions: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	data = append(data, '\n')
	if err := storage.OS.WriteFile(s.Path(dir), data, 0o644); err != nil {
		return fmt.Errorf("write session: %w", err)
	}
	return nil
}

// LoadSession reads a session by ID from the directory.
func LoadSession(dir, id string) (*Session, error) {
	data, err := storage.OS.ReadFile(filepath.Join(dir, id+".json"))
	if err != nil {
		return nil, fmt.Errorf("read session %s: %w", id, err)
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse session %s: %w", id, err)
	}
	return &s, nil
}

// generateSessionID returns a UUIDv4-style ID.
func generateSessionID() string {
	return "s-" + uuid.NewString()
}
