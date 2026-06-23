package expand

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/iannil/jianwu/internal/corpus"
	"github.com/iannil/jianwu/internal/provider/llm"
)

// LookupSimilarBooks embeds the query and all builtin corpus chunks,
// returns top-N matching book slugs. Per Q20.2: real-time, no pre-built index.
func LookupSimilarBooks(ctx context.Context, embedder llm.Embedder, query string, topN int) ([]string, error) {
	if embedder == nil {
		return nil, fmt.Errorf("embedder not configured")
	}
	// 1. Embed query.
	qResp, err := embedder.Embed(ctx, llm.EmbedRequest{Inputs: []string{query}})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(qResp.Embeddings) == 0 {
		return nil, fmt.Errorf("empty embedding for query")
	}
	queryVec := qResp.Embeddings[0]

	// 2. Load corpus + embed each book's abstract.
	books, err := corpus.Load()
	if err != nil {
		return nil, fmt.Errorf("load corpus: %w", err)
	}
	var inputs []string
	var slugs []string
	for slug, bk := range books {
		// Use the book's abstract as the embedding text.
		text := bk.Abstract
		if text == "" {
			text = bk.Title.Zh + " " + bk.Title.En
		}
		inputs = append(inputs, text)
		slugs = append(slugs, slug)
	}
	if len(inputs) == 0 {
		return nil, nil
	}

	bResp, err := embedder.Embed(ctx, llm.EmbedRequest{Inputs: inputs})
	if err != nil {
		return nil, fmt.Errorf("embed corpus: %w", err)
	}

	// 3. Compute cosine similarity + sort.
	type scored struct {
		slug  string
		score float64
	}
	var ranks []scored
	for i, vec := range bResp.Embeddings {
		s := cosine(queryVec, vec)
		ranks = append(ranks, scored{slug: slugs[i], score: s})
	}
	sort.Slice(ranks, func(i, j int) bool {
		return ranks[i].score > ranks[j].score
	})

	// 4. Return top-N slugs.
	if topN > len(ranks) {
		topN = len(ranks)
	}
	out := make([]string, topN)
	for i := 0; i < topN; i++ {
		out[i] = ranks[i].slug
	}
	return out, nil
}

// cosine computes cosine similarity between two vectors.
func cosine(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		af := float64(a[i])
		bf := float64(b[i])
		dot += af * bf
		na += af * af
		nb += bf * bf
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// LookupSimilarBook calls the free function with per-chapter cap 2 (per Q13.A2).
// Returns top-3 results per call.
func (t *ToolRegistry) LookupSimilarBook(ctx context.Context, query string) ([]string, error) {
	t.mu.Lock()
	if t.lookupSimilarCalls >= 2 {
		t.mu.Unlock()
		return nil, fmt.Errorf("lookup_similar_book call limit (2) reached")
	}
	t.lookupSimilarCalls++
	t.mu.Unlock()
	return LookupSimilarBooks(ctx, t.Embedder, query, 3)
}
