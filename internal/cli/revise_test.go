package cli

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/storage"
)

func TestReviseRebuildsEvidenceAndClearsReview(t *testing.T) {
	tmp := writeMinimalBook(t, "demo")
	dir := filepath.Join(tmp, "books", "demo")
	o, err := book.LoadOutline(filepath.Join(dir, "outline.json"))
	if err != nil {
		t.Fatal(err)
	}
	c := &o.Parts[0].Chapters[0]
	now := time.Now()
	c.Status = book.StatusReviewed
	c.ReviewedAt = &now
	c.ReviewedBy = "reviewer"
	c.Claims = []book.Claim{{Text: "old", HasCitation: true, CitationIDs: []string{"1"}}}
	c.Citations = []book.Citation{{ID: "1", URL: "https://old.example"}}
	c.Verdicts = []book.ClaimVerdict{{ClaimText: "old", Verified: true}}
	if err := book.SaveOutline(filepath.Join(dir, "outline.json"), o); err != nil {
		t.Fatal(err)
	}
	_, err = book.WriteChapter(dir, 1, 1, book.ChapterFrontmatter{Status: book.StatusReviewed, Model: "original-model", EngineVersion: "v-original"}, "old")
	if err != nil {
		t.Fatal(err)
	}
	chdir(t, tmp)
	p := &countingChatter{responses: []llm.ChatResponse{
		{Content: "intermediate"},
		{Content: `{"revised_markdown":"new claim[^9]\n\n[^9]: [New](https://new.example)","claims":[{"text":"new claim","citation_ids":["9"],"has_citation":true},{"text":"unlinked","citation_ids":[],"has_citation":false}]}`},
	}}
	cmd := newReviseCmd()
	cmd.SetContext(context.Background())
	cmd.SetOut(&bytes.Buffer{})
	if err := runReviseWithDeps(cmd, []string{"demo", "1-1"}, &ProviderDeps{Chatter: p}); err != nil {
		t.Fatal(err)
	}
	o, err = book.LoadOutline(filepath.Join(dir, "outline.json"))
	if err != nil {
		t.Fatal(err)
	}
	c = &o.Parts[0].Chapters[0]
	if c.Status != book.StatusExpanded || c.ReviewedAt != nil || c.ReviewedBy != "" || len(c.Verdicts) != 0 {
		t.Fatalf("stale review: %+v", c)
	}
	if len(c.Claims) != 2 || c.Claims[0].Text != "new claim" || c.Claims[0].CitationIDs[0] != "9" || c.UnverifiedClaims != 1 {
		t.Fatalf("claims: %+v", c)
	}
	if len(c.Citations) != 1 || c.Citations[0].ID != "9" {
		t.Fatalf("citations: %+v", c.Citations)
	}
	fm, body, err := book.ReadChapter(book.ChapterPath(dir, 1, 1))
	if err != nil {
		t.Fatal(err)
	}
	if fm.Model != "original-model" || fm.EngineVersion != "v-original" || fm.Status != book.StatusExpanded || fm.UnverifiedClaimsCount != 1 || len(fm.Citations) != 1 || body == "intermediate" {
		t.Fatalf("frontmatter/body: %+v %s", fm, body)
	}
}

func TestFactcheckLegacyClaimsWithoutSourcesAreVisible(t *testing.T) {
	tmp := writeMinimalBook(t, "demo")
	dir := filepath.Join(tmp, "books", "demo")
	o, err := book.LoadOutline(filepath.Join(dir, "outline.json"))
	if err != nil {
		t.Fatal(err)
	}
	o.Parts[0].Chapters[0].Claims = []book.Claim{{Text: "legacy", HasCitation: true}}
	if err := book.SaveOutline(filepath.Join(dir, "outline.json"), o); err != nil {
		t.Fatal(err)
	}
	chdir(t, tmp)
	cmd := newFactCheckCmd()
	cmd.SetContext(context.Background())
	cmd.SetOut(&bytes.Buffer{})
	if err := runFactCheckWithDeps(cmd, []string{"demo", "1-1"}, &ProviderDeps{}); err != nil {
		t.Fatal(err)
	}
	o, err = book.LoadOutline(filepath.Join(dir, "outline.json"))
	if err != nil {
		t.Fatal(err)
	}
	vs := o.Parts[0].Chapters[0].Verdicts
	if len(vs) != 1 || vs[0].Verified {
		t.Fatalf("legacy silently skipped: %+v", vs)
	}
}

func TestReviseValidationFailureLeavesChapterUntouched(t *testing.T) {
	tmp := writeMinimalBook(t, "demo")
	dir := filepath.Join(tmp, "books", "demo")
	_, err := book.WriteChapter(dir, 1, 1, book.ChapterFrontmatter{Status: book.StatusReviewed, Model: "original"}, "original prose")
	if err != nil {
		t.Fatal(err)
	}
	chdir(t, tmp)
	p := &countingChatter{responses: []llm.ChatResponse{{Content: "rewrite"}, {Content: "invalid JSON"}}}
	cmd := newReviseCmd()
	cmd.SetContext(context.Background())
	cmd.SetOut(&bytes.Buffer{})
	if err := runReviseWithDeps(cmd, []string{"demo", "1-1"}, &ProviderDeps{Chatter: p}); err == nil {
		t.Fatal("expected validation failure")
	}
	fm, body, err := book.ReadChapter(book.ChapterPath(dir, 1, 1))
	if err != nil {
		t.Fatal(err)
	}
	if fm.Status != book.StatusReviewed || !strings.Contains(body, "original prose") {
		t.Fatalf("failed validation overwrote chapter: %+v %q", fm, body)
	}
}

func TestReviseRejectsBlankProse(t *testing.T) {
	for _, tc := range []struct {
		name      string
		responses []llm.ChatResponse
	}{
		{"revision", []llm.ChatResponse{{Content: " \n\t"}}},
		{"validation", []llm.ChatResponse{{Content: "rewrite"}, {Content: `{"revised_markdown":" \n\t","claims":[]}`}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmp := writeMinimalBook(t, "demo")
			dir := filepath.Join(tmp, "books", "demo")
			if _, err := book.WriteChapter(dir, 1, 1, book.ChapterFrontmatter{Status: book.StatusExpanded}, "original prose"); err != nil {
				t.Fatal(err)
			}
			chdir(t, tmp)
			cmd := newReviseCmd()
			cmd.SetContext(context.Background())
			cmd.SetOut(&bytes.Buffer{})
			if err := runReviseWithDeps(cmd, []string{"demo", "1-1"}, &ProviderDeps{Chatter: &countingChatter{responses: tc.responses}}); err == nil {
				t.Fatal("accepted blank prose")
			}
			_, body, err := book.ReadChapter(book.ChapterPath(dir, 1, 1))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(body, "original prose") {
				t.Fatalf("blank prose overwrote chapter: %q", body)
			}
		})
	}
}

type failRevisionStorage struct {
	storage.Storage
	target string
	failed bool
}

func (s *failRevisionStorage) Rename(a, b string) error {
	if !s.failed && strings.HasSuffix(b, s.target) {
		s.failed = true
		return errors.New("injected revision save failure")
	}
	return s.Storage.Rename(a, b)
}

func TestReviseSaveFailureRestoresReviewAndRetainsUsage(t *testing.T) {
	for _, target := range []string{"outline.json", "meta.json"} {
		t.Run(target, func(t *testing.T) {
			tmp := writeMinimalBook(t, "demo")
			dir := filepath.Join(tmp, "books", "demo")
			o, err := book.LoadOutline(filepath.Join(dir, "outline.json"))
			if err != nil {
				t.Fatal(err)
			}
			c := &o.Parts[0].Chapters[0]
			now := time.Now()
			c.Status = book.StatusReviewed
			c.ReviewedAt = &now
			c.ReviewedBy = "reader"
			c.Verdicts = []book.ClaimVerdict{{ClaimText: "original", Verified: true}}
			if err := book.SaveOutline(filepath.Join(dir, "outline.json"), o); err != nil {
				t.Fatal(err)
			}
			if _, err := book.WriteChapter(dir, 1, 1, book.ChapterFrontmatter{Status: book.StatusReviewed}, "original prose"); err != nil {
				t.Fatal(err)
			}
			original, err := book.DefaultStorage.ReadFile(filepath.Join(dir, "outline.json"))
			if err != nil {
				t.Fatal(err)
			}
			old := book.DefaultStorage
			book.DefaultStorage = &failRevisionStorage{Storage: old, target: target}
			t.Cleanup(func() { book.DefaultStorage = old })
			chdir(t, tmp)
			p := &countingChatter{responses: []llm.ChatResponse{
				{Content: "rewritten", Usage: llm.Usage{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5}},
				{Content: `{"revised_markdown":"rewritten","claims":[]}`, Usage: llm.Usage{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5}},
			}}
			cmd := newReviseCmd()
			cmd.SetContext(context.Background())
			cmd.SetOut(&bytes.Buffer{})
			if err := runReviseWithDeps(cmd, []string{"demo", "1-1"}, &ProviderDeps{Chatter: p}); err == nil {
				t.Fatal("expected save failure")
			}
			fm, body, err := book.ReadChapter(book.ChapterPath(dir, 1, 1))
			if err != nil {
				t.Fatal(err)
			}
			if fm.Status != book.StatusReviewed || body != "original prose" {
				t.Fatalf("chapter not restored: %+v %q", fm, body)
			}
			got, err := book.DefaultStorage.ReadFile(filepath.Join(dir, "outline.json"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, original) {
				t.Fatal("review/verdict state not restored")
			}
			meta, err := book.LoadMeta(filepath.Join(dir, "meta.json"))
			if err != nil {
				t.Fatal(err)
			}
			if meta.TokenUsage.CallCount != 2 || meta.TokenUsage.TotalTokens != 10 {
				t.Fatalf("failed revision usage lost: %+v", meta.TokenUsage)
			}
		})
	}
}
