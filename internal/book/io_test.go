package book

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveAndLoadMetaRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "meta.json")

	original := &Meta{
		ID:        "018f3d3a-1b2c-7d3e-9a4b-1234567890ab",
		Slug:      "reality-of-time",
		Title:     "时间的实在",
		Archetype: "ontology-epistemology-practice",
		Language:  "zh",
		Status:    "draft",
		CreatedAt: time.Date(2026, 6, 21, 14, 30, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 6, 21, 14, 30, 0, 0, time.UTC),
		Parameters: Parameters{
			Audience: "educated-general",
			Depth:    "advanced",
			Goal:     "understanding",
			Length:   "long",
		},
	}
	if err := SaveMeta(path, original); err != nil {
		t.Fatalf("SaveMeta: %v", err)
	}

	// Verify file is valid JSON
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("meta.json is empty")
	}

	loaded, err := LoadMeta(path)
	if err != nil {
		t.Fatalf("LoadMeta: %v", err)
	}
	if loaded.ID != original.ID {
		t.Errorf("ID: got %q want %q", loaded.ID, original.ID)
	}
	if loaded.Title != original.Title {
		t.Errorf("Title: got %q want %q", loaded.Title, original.Title)
	}
	if !loaded.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt mismatch: got %v want %v", loaded.CreatedAt, original.CreatedAt)
	}
}

func TestSaveAndLoadOutlineRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "outline.json")

	original := &Outline{
		Parts: []OutlinePart{
			{
				Index: 1, Title: "第一部", Role: "ontology",
				Chapters: []OutlineChapter{
					{Index: 1, Title: "第一章", Status: StatusScaffolded},
				},
			},
		},
	}
	if err := SaveOutline(path, original); err != nil {
		t.Fatalf("SaveOutline: %v", err)
	}

	loaded, err := LoadOutline(path)
	if err != nil {
		t.Fatalf("LoadOutline: %v", err)
	}
	if len(loaded.Parts) != 1 {
		t.Fatalf("parts: got %d want 1", len(loaded.Parts))
	}
	if loaded.Parts[0].Chapters[0].Status != StatusScaffolded {
		t.Errorf("status: got %q want %q", loaded.Parts[0].Chapters[0].Status, StatusScaffolded)
	}
}

func TestLoadMetaMissingFileReturnsError(t *testing.T) {
	_, err := LoadMeta("/nonexistent/meta.json")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}
