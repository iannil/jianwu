package server

import (
	"testing"

	"github.com/iannil/jianwu/internal/book"
)

// expandInputFor must carry the outline-planned per-chapter word target into
// the expand engine input (MCP tools share this orchestration path).
func TestExpandInputForCarriesWordTarget(t *testing.T) {
	bc := &bookCtx{
		Meta: &book.Meta{
			Title:     "测试书",
			Archetype: "micro-meso-macro",
			Parameters: book.Parameters{
				Audience: "beginner", Depth: "intermediate", Goal: "understanding", Length: "medium",
			},
			Language: "zh",
		},
		Outline: &book.Outline{
			Parts: []book.OutlinePart{
				{Index: 1, Title: "P1", Role: "micro", Chapters: []book.OutlineChapter{
					{Index: 1, Title: "C1", Abstract: "a", KeyConcepts: []string{"k"}, WordCountTarget: 3200},
					{Index: 2, Title: "C2", Abstract: "b", KeyConcepts: []string{"k2"}},
				}},
			},
		},
	}
	in, err := expandInputFor(bc, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if in.WordCountTarget != 3200 {
		t.Errorf("planned chapter: WordCountTarget = %d, want 3200", in.WordCountTarget)
	}
	in, err = expandInputFor(bc, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if in.WordCountTarget != 0 {
		t.Errorf("unplanned chapter: WordCountTarget = %d, want 0", in.WordCountTarget)
	}
}
