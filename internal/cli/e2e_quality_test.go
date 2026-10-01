package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/provider/llm"
	"github.com/iannil/jianwu/internal/provider/llm/mock"
	"github.com/iannil/jianwu/internal/provider/reader"
)

type qualityReader struct{ urls []string }

func (r *qualityReader) Read(_ context.Context, url string) (reader.Content, error) {
	r.urls = append(r.urls, url)
	return reader.Content{URL: url, Markdown: "The measured value is 20."}, nil
}

func TestE2EQualityPipeline(t *testing.T) {
	dir := setupBatchBook(t, 1)
	command := func(cmd *cobra.Command) *cobra.Command {
		cmd.SetContext(context.Background())
		cmd.SetOut(&bytes.Buffer{})
		return cmd
	}
	args := []string{"batch", "1-1"}
	load := func() book.OutlineChapter {
		t.Helper()
		o, err := book.LoadOutline(filepath.Join(dir, "outline.json"))
		if err != nil {
			t.Fatal(err)
		}
		return o.Parts[0].Chapters[0]
	}
	p := &countingChatter{responses: []llm.ChatResponse{
		{Content: `{"findings":[],"candidates":[]}`},
		{Content: "The measured value is 10.[^4]\n\n[^4]: [Original](https://example.com/original)"},
		{Content: `{"revised_markdown":"The measured value is 10.[^4]\n\n[^4]: [Original](https://example.com/original)","claims":[{"text":"The measured value is 10.","has_citation":true,"citation_ids":["4"]}]}`},
	}}
	if err := runExpand(command(newExpandCmd()), args, 0, &ProviderDeps{Chatter: p, Searcher: &stubSearcher{}, Reader: &stubReader{}, Embedder: &stubEmbedder{}}, false); err != nil {
		t.Fatal(err)
	}
	c := load()
	if len(c.Claims) != 1 || c.Claims[0].CitationIDs[0] != "4" {
		t.Fatalf("expand lost claim: %+v", c)
	}
	rd := &qualityReader{}
	verify := mock.New(llm.ChatResponse{Content: `{"verified":false,"reasoning":"Source says 20, not 10","suggested_rewrite":"The measured value is 20."}`})
	if err := runFactCheckWithDeps(command(newFactCheckCmd()), args, &ProviderDeps{Chatter: verify, Reader: rd}); err != nil {
		t.Fatal(err)
	}
	if len(rd.urls) != 1 || rd.urls[0] != "https://example.com/original" || !strings.Contains(verify.Calls()[0].Messages[1].Content, "The measured value is 10.") {
		t.Fatal("factcheck lost claim/source association")
	}
	c = load()
	if len(c.Verdicts) != 1 || c.Verdicts[0].Verified {
		t.Fatalf("missing failed verdict: %+v", c)
	}
	if err := runReview(command(newReviewCmd()), args); err != nil {
		t.Fatal(err)
	}
	// Existing books can contain stale final metadata after a chapter is reopened.
	meta, err := book.LoadMeta(filepath.Join(dir, "meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	meta.Status = book.BookStatusFinal
	if err := book.SaveMeta(filepath.Join(dir, "meta.json"), meta); err != nil {
		t.Fatal(err)
	}
	p = &countingChatter{responses: []llm.ChatResponse{
		{Content: "The measured value is 20.[^9]\n\n[^9]: [Corrected](https://example.com/corrected)"},
		{Content: `{"revised_markdown":"The measured value is 20.[^9]\n\n[^9]: [Corrected](https://example.com/corrected)","claims":[{"text":"The measured value is 20.","has_citation":true,"citation_ids":["9"]}]}`},
	}}
	if err := runReviseWithDeps(command(newReviseCmd()), args, &ProviderDeps{Chatter: p}); err != nil {
		t.Fatal(err)
	}
	c = load()
	if c.Status != book.StatusExpanded || c.ReviewedAt != nil || len(c.Verdicts) != 0 || c.Claims[0].CitationIDs[0] != "9" {
		t.Fatalf("stale revised state: %+v", c)
	}
	meta, err = book.LoadMeta(filepath.Join(dir, "meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	if meta.Status != book.BookStatusDraft {
		t.Fatalf("revision left final book: %s", meta.Status)
	}
	verify = mock.New(llm.ChatResponse{Content: `{"verified":true,"reasoning":"Source states 20"}`})
	if err := runFactCheckWithDeps(command(newFactCheckCmd()), args, &ProviderDeps{Chatter: verify, Reader: rd}); err != nil {
		t.Fatal(err)
	}
	c = load()
	if len(c.Verdicts) != 1 || !c.Verdicts[0].Verified || c.Verdicts[0].CitationID != "9" || rd.urls[1] != "https://example.com/corrected" || !strings.Contains(verify.Calls()[0].Messages[1].Content, "The measured value is 20.") {
		t.Fatalf("recheck used stale evidence: %+v", c)
	}
	if err := runReview(command(newReviewCmd()), args); err != nil {
		t.Fatal(err)
	}
	if err := runFinalize(command(newFinalizeCmd()), []string{"batch"}, false); err != nil {
		t.Fatal(err)
	}
	if err := runExport(command(newExportCmd()), []string{"batch"}, "md", false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "export", "batch.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"The measured value is 20.[^1]", "[^1]: [Corrected](https://example.com/corrected)", `status: "final"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("export missing %q: %s", want, data)
		}
	}
	if strings.Contains(string(data), "The measured value is 10.") {
		t.Fatal("export retained old prose")
	}
}
