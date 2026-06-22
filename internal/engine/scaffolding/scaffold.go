package scaffolding

import (
	"context"
	"fmt"
	"sync"

	"github.com/zhurong/jianwu/internal/book"
	"github.com/zhurong/jianwu/internal/provider/llm"
	"golang.org/x/sync/errgroup"
)

// Options controls parallel scaffolding behavior.
type Options struct {
	// Concurrency limits parallel LLM calls. Default 5 per Q12.A1.
	Concurrency int
}

// Result captures the outcome of scaffolding one chapter.
type Result struct {
	PartIndex    int
	ChapterIndex int
	Chapter      *ChapterOutput
	Err          error
}

// ScaffoldAll runs GenerateChapter for every chapter in the outline, in parallel.
// Returns a map keyed by "partIndex-chapterIndex" with each chapter's result.
// Chapters that succeed have their book.OutlineChapter fields populated (in-place update).
// Chapters that fail have status=failed set on the outline entry and Err set in the result map.
//
// Per Q12.B2: continue-on-error. One failure does NOT abort other chapters.
func ScaffoldAll(
	ctx context.Context,
	chatter llm.Chatter,
	outline *book.Outline,
	archetypeID string,
	params ChapterParams,
	opts Options,
) map[string]Result {
	if opts.Concurrency <= 0 {
		opts.Concurrency = 5
	}

	// Collect all chapter inputs up-front.
	type job struct {
		key     string
		partIdx int
		chIdx   int
		input   ChapterInput
	}
	var jobs []job
	for _, p := range outline.Parts {
		for _, c := range p.Chapters {
			input := ChapterInput{
				ArchetypeID:  archetypeID,
				PartIndex:    p.Index,
				PartTitle:    p.Title,
				PartRole:     p.Role,
				ChapterIndex: c.Index,
				ChapterTitle: c.Title,
				Topic:        params.Topic,
				Audience:     params.Audience,
				Depth:        params.Depth,
				Goal:         params.Goal,
				Length:       params.Length,
				Language:     params.Language,
			}
			jobs = append(jobs, job{
				key:     fmtKey(p.Index, c.Index),
				partIdx: p.Index,
				chIdx:   c.Index,
				input:   input,
			})
		}
	}

	results := make(map[string]Result, len(jobs))
	var mu sync.Mutex

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(opts.Concurrency)

	for _, j := range jobs {
		j := j
		g.Go(func() error {
			// Note: we use gctx for cancellation propagation but each chapter
			// still attempts even if a sibling failed (continue-on-error).
			// errgroup normally cancels on first error; we work around this by
			// always returning nil from g.Go (errors are captured per-chapter).
			chapCtx := gctx
			// If errgroup already cancelled due to context cancel, skip.
			if err := gctx.Err(); err != nil {
				mu.Lock()
				results[j.key] = Result{PartIndex: j.partIdx, ChapterIndex: j.chIdx, Err: err}
				mu.Unlock()
				return nil
			}

			out, err := GenerateChapter(chapCtx, chatter, j.input)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				results[j.key] = Result{
					PartIndex: j.partIdx, ChapterIndex: j.chIdx, Err: err,
				}
				return nil // don't propagate — continue-on-error
			}
			results[j.key] = Result{
				PartIndex: j.partIdx, ChapterIndex: j.chIdx, Chapter: out,
			}
			return nil
		})
	}
	_ = g.Wait()

	// Apply successful results back to the outline.
	for i := range outline.Parts {
		for j := range outline.Parts[i].Chapters {
			c := &outline.Parts[i].Chapters[j]
			key := fmtKey(outline.Parts[i].Index, c.Index)
			r, ok := results[key]
			if !ok {
				continue
			}
			if r.Err != nil {
				c.Status = book.StatusFailed
				continue
			}
			c.Abstract = r.Chapter.Abstract
			c.KeyConcepts = r.Chapter.KeyConcepts
			c.LearningObjectives = r.Chapter.LearningObjectives
			c.SuggestedExamples = r.Chapter.SuggestedExamples
			c.Status = book.StatusScaffolded
		}
	}
	return results
}

// ChapterParams is the book-level context (topic, audience, depth, goal, length, language).
type ChapterParams struct {
	Topic    string
	Audience string
	Depth    string
	Goal     string
	Length   string
	Language string
}

func fmtKey(partIdx, chIdx int) string {
	return fmt.Sprintf("%d-%d", partIdx, chIdx)
}
