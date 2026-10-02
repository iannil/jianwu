package server

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/iannil/jianwu/internal/book"
)

// mcpTestSession connects an MCP client to the server over in-memory
// transports and returns call helpers.
func mcpTestSession(t *testing.T, srv *Server) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	go func() { _ = srv.MCP().Run(ctx, st) }()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// callTool invokes a tool and decodes its JSON text payload.
func callTool[T any](t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) (T, error) {
	t.Helper()
	var out T
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return out, err
	}
	if len(res.Content) == 0 {
		t.Fatalf("tool %s: empty content", name)
	}
	txt, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("tool %s: non-text content %T", name, res.Content[0])
	}
	if err := json.Unmarshal([]byte(txt.Text), &out); err != nil {
		t.Fatalf("tool %s: not JSON: %v\n%s", name, err, txt.Text)
	}
	return out, nil
}

func TestMCP_ListsTools(t *testing.T) {
	srv, _ := newTestEnv(t)
	cs := mcpTestSession(t, srv)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"workspace_status": false, "list_books": false, "book_status": false,
		"read_chapter": false, "create_book": false, "expand_chapter": false,
		"expand_all": false, "factcheck": false, "revise": false,
		"review_chapter": false, "finalize": false, "export_book": false,
		"publish": false, "list_releases": false, "generate_site": false,
		"job_status": false,
	}
	for _, tool := range res.Tools {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("tool %s not registered", name)
		}
	}
}

func TestMCP_BookStatusAndPublishGate(t *testing.T) {
	srv, _ := newTestEnv(t)
	writePublishableFixture(t, srv.WSRoot(), "life")
	cs := mcpTestSession(t, srv)

	type bookStatus struct {
		Slug       string `json:"slug"`
		Title      string `json:"title"`
		Status     string `json:"status"`
		LicenseSet bool   `json:"license_set"`
		Chapters   []any  `json:"chapters"`
	}
	st, err := callTool[bookStatus](t, cs, "book_status", map[string]any{"slug": "life"})
	if err != nil {
		t.Fatal(err)
	}
	if st.Slug != "life" || !st.LicenseSet || len(st.Chapters) != 2 {
		t.Errorf("book_status = %+v", st)
	}

	// publish dry_run reports gate + version synchronously.
	type gate struct {
		Version string `json:"version"`
		Gate    struct {
			Blockers []string `json:"blockers"`
			Warnings []string `json:"warnings"`
		} `json:"gate"`
	}
	g, err := callTool[gate](t, cs, "publish", map[string]any{"slug": "life", "dry_run": true})
	if err != nil {
		t.Fatal(err)
	}
	if g.Version != "1.0" || len(g.Gate.Blockers) != 0 {
		t.Errorf("publish dry-run = %+v", g)
	}
}

func TestMCP_ReviewGateRequiresReviewer(t *testing.T) {
	srv, _ := newTestEnv(t)
	// A book with one expanded chapter (reviewable).
	bookDir := createBookFixture(t, srv.WSRoot(), "life", 1)
	o, err := loadOutlineAt(filepath.Join(bookDir, "outline.json"))
	if err != nil {
		t.Fatal(err)
	}
	for pi := range o.Parts {
		for ci := range o.Parts[pi].Chapters {
			c := o.Parts[pi].Chapters[ci]
			o.Parts[pi].Chapters[ci].Status = book.StatusExpanded
			if _, err := book.WriteChapter(bookDir, 1, c.Index, book.ChapterFrontmatter{
				Title: c.Title, PartIndex: 1, ChapterIndex: c.Index, Status: book.StatusExpanded,
			}, "正文"); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := saveOutlineAt(filepath.Join(bookDir, "outline.json"), o); err != nil {
		t.Fatal(err)
	}

	cs := mcpTestSession(t, srv)
	args := map[string]any{"slug": "life", "part": 1, "chapter": 1}

	// Without reviewer → tool error carrying the gate message.
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "review_chapter", Arguments: args,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError || !resultText(res, "reviewer is required") {
		t.Errorf("expected reviewer-required tool error, got IsError=%v content=%v", res.IsError, res.Content)
	}

	// With reviewer → recorded, outline status flips to reviewed.
	res2, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "review_chapter",
		Arguments: map[string]any{"slug": "life", "part": 1, "chapter": 1, "reviewer": "agent:test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res2.IsError || !resultText(res2, "reviewed life 01-01") {
		t.Errorf("review call failed: IsError=%v content=%v", res2.IsError, res2.Content)
	}
	o2, _ := loadOutlineAt(filepath.Join(bookDir, "outline.json"))
	if o2.Parts[0].Chapters[0].Status != book.StatusReviewed || o2.Parts[0].Chapters[0].ReviewedBy != "agent:test" {
		t.Errorf("review not recorded: %+v", o2.Parts[0].Chapters[0])
	}
}

func TestMCP_ExportJobRoundTrip(t *testing.T) {
	srv, _ := newTestEnv(t)
	writePublishableFixture(t, srv.WSRoot(), "life")
	cs := mcpTestSession(t, srv)

	started, err := callTool[struct {
		JobID string `json:"job_id"`
	}](t, cs, "export_book", map[string]any{"slug": "life", "target": "md"})
	if err != nil {
		t.Fatal(err)
	}
	if started.JobID == "" {
		t.Fatal("export_book must return job_id")
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		st, err := callTool[struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		}](t, cs, "job_status", map[string]any{"job_id": started.JobID})
		if err != nil {
			t.Fatal(err)
		}
		if st.Status == "succeeded" {
			return
		}
		if st.Status == "failed" {
			t.Fatalf("export job failed: %s", st.Error)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("job did not finish in time")
}

// resultText reports whether any text content in the result contains sub.
func resultText(res *mcp.CallToolResult, sub string) bool {
	for _, c := range res.Content {
		if txt, ok := c.(*mcp.TextContent); ok && strings.Contains(txt.Text, sub) {
			return true
		}
	}
	return false
}

// loadOutlineAt / saveOutlineAt are small test shims over book IO.
func loadOutlineAt(path string) (*book.Outline, error) { return book.LoadOutline(path) }
func saveOutlineAt(path string, o *book.Outline) error { return book.SaveOutline(path, o) }
