package corpus

import (
	"context"
	"math"
	"path/filepath"
	"testing"

	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/storage"
)

// mockEmbedder returns deterministic embeddings for tests.
type mockEmbedder struct {
	dim int
}

func (m *mockEmbedder) Embed(_ context.Context, req llm.EmbedRequest) (*llm.EmbedResponse, error) {
	out := make([][]float32, len(req.Inputs))
	for i := range req.Inputs {
		vec := make([]float32, m.dim)
		// Deterministic: use hash of input
		h := hashString(req.Inputs[i])
		for d := 0; d < m.dim; d++ {
			vec[d] = float32(h>>(d*5%60)) / float32(math.MaxUint32)
		}
		out[i] = vec
	}
	return &llm.EmbedResponse{Embeddings: out}, nil
}

func hashString(s string) uint64 {
	var h uint64 = 14695981039346656037
	for _, c := range []byte(s) {
		h ^= uint64(c)
		h *= 1099511628211
	}
	return h
}

func TestBuildIndex(t *testing.T) {
	books := map[string]*Book{
		"book-a": {
			Slug:     "book-a",
			Title:    LocalizedTitle{Zh: "图书A", En: "Book A"},
			Abstract: "这是图书A的摘要",
			Parts: []Part{
				{Index: 1, Title: LocalizedTitle{Zh: "第一部"}, Chapters: []Chapter{
					{Index: 1, Title: LocalizedTitle{Zh: "第一章"}},
				}},
			},
		},
		"book-b": {
			Slug:     "book-b",
			Title:    LocalizedTitle{Zh: "图书B", En: "Book B"},
			Abstract: "这是图书B的摘要",
		},
	}

	embedder := &mockEmbedder{dim: 4}
	idx, err := BuildIndex(context.Background(), embedder, "mock-model", books)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}

	if idx.Version != IndexSchemaVersion {
		t.Errorf("expected version %d, got %d", IndexSchemaVersion, idx.Version)
	}
	if idx.EmbeddingModel != "mock-model" {
		t.Errorf("expected model mock-model, got %s", idx.EmbeddingModel)
	}
	if idx.Dim != 4 {
		t.Errorf("expected dim 4, got %d", idx.Dim)
	}
	if len(idx.Books) != 2 {
		t.Fatalf("expected 2 books, got %d", len(idx.Books))
	}
	if idx.Books[0].Slug != "book-a" {
		t.Errorf("expected first book book-a, got %s", idx.Books[0].Slug)
	}
	if len(idx.Books[0].Embedding) != 4 {
		t.Errorf("expected embedding dim 4, got %d", len(idx.Books[0].Embedding))
	}
}

func TestSaveLoadIndex(t *testing.T) {
	idx := &CorpusIndex{
		Version:        IndexSchemaVersion,
		EmbeddingModel: "test-model",
		Dim:            3,
		Books: []IndexedBook{
			{Slug: "a", Title: "A", Embedding: []float32{0.1, 0.2, 0.3}},
			{Slug: "b", Title: "B", Embedding: []float32{0.4, 0.5, 0.6}},
		},
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "index.json")
	if err := SaveIndex(path, idx); err != nil {
		t.Fatalf("SaveIndex: %v", err)
	}

	loaded, err := LoadIndex(path)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}

	if loaded.Version != IndexSchemaVersion {
		t.Errorf("version mismatch")
	}
	if loaded.EmbeddingModel != "test-model" {
		t.Errorf("model mismatch")
	}
	if len(loaded.Books) != 2 {
		t.Fatalf("expected 2 books, got %d", len(loaded.Books))
	}
	if loaded.Books[0].Slug != "a" {
		t.Errorf("expected slug a, got %s", loaded.Books[0].Slug)
	}
}

func TestLoadIndexWrongVersion(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "bad.json")
	badData := `{"version": 999, "books": []}`
	if err := storage.OS.WriteFile(path, []byte(badData), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadIndex(path)
	if err == nil {
		t.Fatal("expected error for unsupported version")
	}
}

func TestFindSimilar(t *testing.T) {
	idx := &CorpusIndex{
		Books: []IndexedBook{
			{Slug: "a", Embedding: []float32{1, 0, 0}},
			{Slug: "b", Embedding: []float32{0.9, 0.1, 0}},
			{Slug: "c", Embedding: []float32{0, 1, 0}},
		},
	}

	// a is most similar to b
	similar := idx.FindSimilar("a", 2)
	if len(similar) != 2 {
		t.Fatalf("expected 2 results, got %d", len(similar))
	}
	if similar[0] != "b" {
		t.Errorf("expected b as most similar to a, got %s", similar[0])
	}
	if similar[1] != "c" {
		t.Errorf("expected c as second, got %s", similar[1])
	}
}

func TestFindSimilarMissing(t *testing.T) {
	idx := &CorpusIndex{
		Books: []IndexedBook{
			{Slug: "a", Embedding: []float32{1, 0}},
		},
	}
	if got := idx.FindSimilar("nonexistent", 1); got != nil {
		t.Errorf("expected nil for missing slug, got %v", got)
	}
}

func TestFindSimilarNilIndex(t *testing.T) {
	var idx *CorpusIndex
	if got := idx.FindSimilar("a", 1); got != nil {
		t.Errorf("expected nil for nil index, got %v", got)
	}
}

func TestEmbedding(t *testing.T) {
	idx := &CorpusIndex{
		Books: []IndexedBook{
			{Slug: "a", Embedding: []float32{0.1, 0.2}},
		},
	}
	if got := idx.Embedding("a"); got == nil || len(got) != 2 || got[0] != 0.1 {
		t.Errorf("unexpected embedding")
	}
	if got := idx.Embedding("b"); got != nil {
		t.Errorf("expected nil for missing")
	}
}

func TestCosineSimilarity(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}
	c := []float32{2, 0, 0} // same direction as a

	if cs := cosineSimilarity(a, b); cs != 0 {
		t.Errorf("orthogonal should be 0, got %f", cs)
	}
	if cs := cosineSimilarity(a, c); cs != 1 {
		t.Errorf("same direction should be 1, got %f", cs)
	}
	if cs := cosineSimilarity(a, a); cs != 1 {
		t.Errorf("same vector should be 1, got %f", cs)
	}
}

func TestTextForEmbedding(t *testing.T) {
	b := &Book{
		Title:    LocalizedTitle{Zh: "测试书", En: "Test Book"},
		Abstract: "摘要",
		Parts: []Part{
			{Title: LocalizedTitle{Zh: "第一部"}, Chapters: []Chapter{
				{Title: LocalizedTitle{Zh: "第一章"}, Abstract: "本章内容"},
			}},
		},
	}
	text := textForEmbedding(b)
	if !contains(text, "测试书") {
		t.Errorf("expected title in text")
	}
	if !contains(text, "摘要") {
		t.Errorf("expected abstract in text")
	}
	if !contains(text, "第一部") {
		t.Errorf("expected part title in text")
	}
	if !contains(text, "第一章") {
		t.Errorf("expected chapter title in text")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
