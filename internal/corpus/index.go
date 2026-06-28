package corpus

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/storage"
)

// IndexSchemaVersion is the current corpus index file format version.
const IndexSchemaVersion = 1

// CorpusIndex is a pre-computed embedding index for corpus books.
// Written to .jianwu/corpus_index.json by `corpus reindex` and loaded by expand.
type CorpusIndex struct {
	Version        int           `json:"version"`
	EmbeddingModel string        `json:"embedding_model"`
	Dim            int           `json:"dim"`
	Books          []IndexedBook `json:"books"`
}

// IndexedBook is one book entry in the index.
type IndexedBook struct {
	Slug      string    `json:"slug"`
	Title     string    `json:"title"`
	Embedding []float32 `json:"embedding"`
}

// SaveIndex writes the index to a file.
func SaveIndex(path string, idx *CorpusIndex) error {
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}
	return storage.OS.WriteFile(path, data, 0o644)
}

// LoadIndex reads the index from a file.
func LoadIndex(path string) (*CorpusIndex, error) {
	data, err := storage.OS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read index: %w", err)
	}
	var idx CorpusIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parse index: %w", err)
	}
	if idx.Version != IndexSchemaVersion {
		return nil, fmt.Errorf("unsupported index version %d (want %d)", idx.Version, IndexSchemaVersion)
	}
	return &idx, nil
}

// textForEmbedding builds a compact text representation of a book for embedding.
func textForEmbedding(b *Book) string {
	var parts []string
	if b.Title.Zh != "" {
		parts = append(parts, b.Title.Zh)
	}
	if b.Title.En != "" {
		parts = append(parts, b.Title.En)
	}
	if b.Abstract != "" {
		parts = append(parts, b.Abstract)
	}
	for _, p := range b.Parts {
		parts = append(parts, p.Title.Zh)
		for _, ch := range p.Chapters {
			parts = append(parts, ch.Title.Zh)
			if ch.Abstract != "" {
				parts = append(parts, ch.Abstract)
			}
		}
	}
	return strings.Join(parts, "\n")
}

// BuildIndex generates embeddings for all corpus books using the given embedder.
// Model name is stored in the index for provenance.
func BuildIndex(ctx context.Context, embedder llm.Embedder, model string, books map[string]*Book) (*CorpusIndex, error) {
	// Collect items sorted by slug for deterministic output
	type item struct {
		slug string
		text string
	}
	var items []item
	for slug, b := range books {
		items = append(items, item{slug: slug, text: textForEmbedding(b)})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].slug < items[j].slug })

	idx := &CorpusIndex{
		Version:        IndexSchemaVersion,
		EmbeddingModel: model,
		Books:          make([]IndexedBook, 0, len(items)),
	}

	for _, it := range items {
		resp, err := embedder.Embed(ctx, llm.EmbedRequest{
			Model:  model,
			Inputs: []string{it.text},
		})
		if err != nil {
			return nil, fmt.Errorf("embed %q: %w", it.slug, err)
		}
		if len(resp.Embeddings) == 0 {
			return nil, fmt.Errorf("embed %q: empty response", it.slug)
		}
		idx.Books = append(idx.Books, IndexedBook{
			Slug:      it.slug,
			Title:     books[it.slug].Title.Zh,
			Embedding: resp.Embeddings[0],
		})
		if idx.Dim == 0 {
			idx.Dim = len(resp.Embeddings[0])
		}
	}

	return idx, nil
}

// cosineSimilarity computes cosine similarity between two vectors.
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(na) * math.Sqrt(nb)))
}

// FindSimilar returns slugs of the topN most similar books to the given slug,
// excluding the query slug itself. Returns nil if slug not found or index is nil.
func (idx *CorpusIndex) FindSimilar(slug string, topN int) []string {
	if idx == nil {
		return nil
	}

	var queryVec []float32
	queryTitle := ""
	for _, b := range idx.Books {
		if b.Slug == slug {
			queryVec = b.Embedding
			queryTitle = b.Title
			break
		}
	}
	if queryVec == nil {
		return nil
	}

	type scored struct {
		slug  string
		score float32
	}
	var results []scored
	for _, b := range idx.Books {
		if b.Slug == slug {
			continue
		}
		score := cosineSimilarity(queryVec, b.Embedding)
		results = append(results, scored{slug: b.Slug, score: score})
	}

	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })
	if topN > len(results) {
		topN = len(results)
	}
	_ = queryTitle // available for future ranking adjustments
	out := make([]string, topN)
	for i := 0; i < topN; i++ {
		out[i] = results[i].slug
	}
	return out
}

// Embedding returns the embedding vector for a given slug, or nil if not found.
func (idx *CorpusIndex) Embedding(slug string) []float32 {
	if idx == nil {
		return nil
	}
	for _, b := range idx.Books {
		if b.Slug == slug {
			return b.Embedding
		}
	}
	return nil
}
