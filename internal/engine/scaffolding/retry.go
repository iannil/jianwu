package scaffolding

import (
	"context"

	"github.com/zhurong/jianwu/internal/book"
	"github.com/zhurong/jianwu/internal/provider/llm"
)

// RetryFailed re-runs GenerateChapter only for chapters whose status is book.StatusFailed.
// It builds a filtered outline containing only failed chapters, calls ScaffoldAll on it,
// then merges results back into the original outline. Returns a result map (same shape as
// ScaffoldAll) for the retried chapters only.
func RetryFailed(
	ctx context.Context,
	chatter llm.Chatter,
	outline *book.Outline,
	archetypeID string,
	params ChapterParams,
	opts Options,
) map[string]Result {
	// Collect failed-chapter jobs.
	type job struct {
		key    string
		partIdx int
		chIdx   int
		input   ChapterInput
	}
	var jobs []job
	for _, p := range outline.Parts {
		for _, c := range p.Chapters {
			if c.Status != book.StatusFailed {
				continue
			}
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
				key:    fmtKey(p.Index, c.Index),
				partIdx: p.Index,
				chIdx:   c.Index,
				input:   input,
			})
		}
	}
	if len(jobs) == 0 {
		return map[string]Result{}
	}

	// Reuse ScaffoldAll's parallel machinery by building a temp outline.
	filtered := &book.Outline{}
	partMap := map[int]*book.OutlinePart{} // tracks part index → pointer in filtered
	for _, p := range outline.Parts {
		// Only include this part if it has at least one failed chapter.
		hasFailed := false
		for _, c := range p.Chapters {
			if c.Status == book.StatusFailed {
				hasFailed = true
				break
			}
		}
		if !hasFailed {
			continue
		}
		fp := book.OutlinePart{Index: p.Index, Title: p.Title, Role: p.Role}
		for _, c := range p.Chapters {
			if c.Status == book.StatusFailed {
				fp.Chapters = append(fp.Chapters, c)
			}
		}
		filtered.Parts = append(filtered.Parts, fp)
		partMap[p.Index] = &filtered.Parts[len(filtered.Parts)-1]
	}

	results := ScaffoldAll(ctx, chatter, filtered, archetypeID, params, opts)

	// Merge filtered results back into the original outline.
	for i := range outline.Parts {
		for j := range outline.Parts[i].Chapters {
			c := &outline.Parts[i].Chapters[j]
			if c.Status != book.StatusFailed {
				continue
			}
			key := fmtKey(outline.Parts[i].Index, c.Index)
			r, ok := results[key]
			if !ok {
				continue
			}
			if r.Err == nil && r.Chapter != nil {
				c.Abstract = r.Chapter.Abstract
				c.KeyConcepts = r.Chapter.KeyConcepts
				c.LearningObjectives = r.Chapter.LearningObjectives
				c.SuggestedExamples = r.Chapter.SuggestedExamples
				c.Status = book.StatusScaffolded
			}
			// else: leave as failed; result map carries the new error
		}
	}
	return results
}
