// Package server exposes RunMCPServer: jianwu as an MCP stdio server
// (ADR 30). Tools reuse the same orchestration as the HTTP API — engine
// calls plus the single-writer JobManager — so agents and humans operate
// under identical constraints and human gates.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/iannil/jianwu/internal/book"
	"github.com/iannil/jianwu/internal/release"
	"github.com/iannil/jianwu/internal/site"
	"github.com/iannil/jianwu/internal/storage"
)

// RunMCPServer runs the jianwu MCP server over stdio until the transport
// closes. Logs go to stderr: stdout belongs to the protocol.
func RunMCPServer(ctx context.Context, wsRoot, version string) error {
	s := New(wsRoot, version, nil)
	return s.MCP().Run(ctx, &mcp.StdioTransport{})
}

// MCP returns the jianwu MCP server bound to this Server instance (one
// server, one serial job queue). Tests drive it over in-memory transports.
func (s *Server) MCP() *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "jianwu", Version: s.version}, nil)
	registerMCPTools(srv, s)
	return srv
}

// textResult builds a plain-text tool result.
func textResult(format string, args ...any) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(format, args...)}}}, nil, nil
}

// jsonResult builds a structured tool result (JSON text; agents parse it).
func jsonResult(v any) (*mcp.CallToolResult, any, error) {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("marshal result: %w", err)
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}}}, nil, nil
}

// registerMCPTools wires the jianwu toolset. Long operations return a job id
// to poll with job_status (same serial queue as the web UI).
func registerMCPTools(srv *mcp.Server, s *Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "workspace_status",
		Description: "Current workspace root, initialization state and book count.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		root := s.WSRoot()
		entries, err := storage.OS.ReadDir(filepath.Join(root, "books"))
		books := 0
		if err == nil {
			for _, e := range entries {
				if e.IsDir() {
					books++
				}
			}
		}
		return jsonResult(map[string]any{"root": root, "initialized": isWorkspaceInit(root), "books": books, "version": s.version})
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_books",
		Description: "All books in the workspace: slug, title, status, chapter progress and token usage.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		root := s.WSRoot()
		entries, err := storage.OS.ReadDir(filepath.Join(root, "books"))
		if err != nil {
			return jsonResult([]any{})
		}
		var out []map[string]any
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			bc, err := s.loadBook(e.Name())
			if err != nil {
				continue
			}
			out = append(out, map[string]any{
				"slug": bc.Meta.Slug, "title": bc.Meta.Title, "status": bc.Meta.Status,
				"parts": len(bc.Outline.Parts), "chapters": mcpChapterCount(bc),
				"words": totalWords(bc), "tokens": bc.Meta.TokenUsage.TotalTokens,
			})
		}
		sort.Slice(out, func(i, j int) bool { return out[i]["slug"].(string) < out[j]["slug"].(string) })
		return jsonResult(out)
	})

	type bookArgs struct {
		Slug string `json:"slug" jsonschema:"book slug from list_books"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "book_status",
		Description: "One book's outline with per-chapter status, word count, citations, unverified claims and verdict summary.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, a bookArgs) (*mcp.CallToolResult, any, error) {
		bc, err := s.loadBook(a.Slug)
		if err != nil {
			return nil, nil, err
		}
		var chapters []map[string]any
		for pi := range bc.Outline.Parts {
			p := &bc.Outline.Parts[pi]
			for ci := range p.Chapters {
				c := &p.Chapters[ci]
				chapters = append(chapters, map[string]any{
					"addr": fmt.Sprintf("%02d-%02d", p.Index, c.Index), "title": c.Title,
					"status": c.Status, "words": c.WordCount, "citations": c.CitationsCount,
					"unverified_claims": c.UnverifiedClaims,
					"failed_verdicts":   failedVerdicts(c.Verdicts),
				})
			}
		}
		return jsonResult(map[string]any{
			"slug": bc.Meta.Slug, "title": bc.Meta.Title, "status": bc.Meta.Status,
			"license_set": bc.Meta.License != "", "author": bc.Meta.Author,
			"tokens": bc.Meta.TokenUsage, "chapters": chapters,
		})
	})

	type readChArgs struct {
		Slug    string `json:"slug"`
		Part    int    `json:"part" jsonschema:"1-based part index"`
		Chapter int    `json:"chapter" jsonschema:"1-based chapter index within the part"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "read_chapter",
		Description: "Full chapter: markdown body with [^N] footnotes, claims, citations and fact-check verdicts (with suggested rewrites).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, a readChArgs) (*mcp.CallToolResult, any, error) {
		bc, err := s.loadBook(a.Slug)
		if err != nil {
			return nil, nil, err
		}
		ch, err := findChapter(bc.Outline, a.Part, a.Chapter)
		if err != nil {
			return nil, nil, err
		}
		_, body, rerr := book.ReadChapter(book.ChapterPath(bc.BookDir, a.Part, a.Chapter))
		if rerr != nil {
			body = "（本章尚未展开）"
		}
		return jsonResult(map[string]any{
			"addr": fmt.Sprintf("%02d-%02d", a.Part, a.Chapter), "title": ch.Title, "status": ch.Status,
			"body": body, "claims": ch.Claims, "citations": ch.Citations, "verdicts": ch.Verdicts,
		})
	})

	type createBookArgs struct {
		Topic     string `json:"topic" jsonschema:"the book's core question or subject"`
		Audience  string `json:"audience,omitempty" jsonschema:"beginner | educated-general | advanced-practitioner | scholar (default beginner)"`
		Goal      string `json:"goal,omitempty" jsonschema:"understanding | operational | decision (default understanding)"`
		Depth     string `json:"depth,omitempty" jsonschema:"intro | intermediate | advanced (default intro)"`
		Length    string `json:"length,omitempty" jsonschema:"short | medium | long (default short)"`
		Language  string `json:"language,omitempty" jsonschema:"zh | en | bilingual (default zh)"`
		Archetype string `json:"archetype,omitempty" jsonschema:"structural archetype; default foundations-application-practice"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name: "create_book",
		Description: "Create a book: records the design dimensions (defaults for unset), generates outline + chapter scaffolds. " +
			"Returns the new slug. Long operation — the tool waits for outline/scaffolding to finish (typically <1 min).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, a createBookArgs) (*mcp.CallToolResult, any, error) {
		shim, err := s.mcpCreateBook(ctx, a.Topic, map[string]string{
			"audience":  a.Audience,
			"goal":      a.Goal,
			"depth":     a.Depth,
			"length":    a.Length,
			"language":  a.Language,
			"archetype": a.Archetype,
		})
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(map[string]any{"slug": shim.slug, "chapters": shim.chapters, "parts": shim.parts, "next": "expand_all or expand_chapter"})
	})

	type chJobArgs struct {
		Slug    string `json:"slug"`
		Part    int    `json:"part"`
		Chapter int    `json:"chapter"`
		Force   bool   `json:"force,omitempty" jsonschema:"regenerate even if already expanded (expand only)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "expand_chapter",
		Description: "Expand one chapter (research → draft → validate) as a background job. Returns job_id; poll with job_status.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, a chJobArgs) (*mcp.CallToolResult, any, error) {
		force := 0
		if a.Force {
			force = 2
		}
		id := s.jobs.Start("expand", a.Slug, func(ctx context.Context, j *Job) error {
			return s.runExpandChapter(ctx, j, a.Slug, a.Part, a.Chapter, force)
		})
		return jobStartedResult(id, "expand_chapter")
	})
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "factcheck",
		Description: "Fact-check one chapter against its cited sources as a background job. Conservative: unreadable sources leave claims unverified — that is not proof of error.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, a chJobArgs) (*mcp.CallToolResult, any, error) {
		id := s.jobs.Start("factcheck", a.Slug, func(ctx context.Context, j *Job) error {
			return s.runFactcheck(ctx, j, a.Slug, a.Part, a.Chapter)
		})
		return jobStartedResult(id, "factcheck")
	})
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "revise",
		Description: "Revise one chapter from its fact-check verdicts as a background job. Invalidates prior review/verdicts: the chapter must be fact-checked and human-reviewed again.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, a chJobArgs) (*mcp.CallToolResult, any, error) {
		id := s.jobs.Start("revise", a.Slug, func(ctx context.Context, j *Job) error {
			return s.runRevise(ctx, j, a.Slug, a.Part, a.Chapter)
		})
		return jobStartedResult(id, "revise")
	})

	type expandAllArgs struct {
		Slug  string `json:"slug"`
		Force bool   `json:"force,omitempty" jsonschema:"re-expand already-expanded chapters too"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "expand_all",
		Description: "Expand every scaffolded/failed chapter as one background job (max 5 concurrent generators, serialized saves). Returns job_id.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, a expandAllArgs) (*mcp.CallToolResult, any, error) {
		force := 0
		if a.Force {
			force = -1
		}
		id := s.jobs.Start("expand", a.Slug, func(ctx context.Context, j *Job) error {
			return s.runExpandAll(ctx, j, a.Slug, force)
		})
		return jobStartedResult(id, "expand_all")
	})

	type reviewArgs struct {
		Slug     string `json:"slug"`
		Part     int    `json:"part"`
		Chapter  int    `json:"chapter"`
		Reviewer string `json:"reviewer,omitempty" jsonschema:"who approves; use an agent identifier like agent:<name>. Required by the handler."`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name: "review_chapter",
		Description: "Mark one chapter as human-approved. GATE: only call this after showing the user the chapter body and its fact-check verdicts " +
			"and receiving explicit approval. Record the operator in reviewer. Review is a human judgment, never a formality.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, a reviewArgs) (*mcp.CallToolResult, any, error) {
		bc, err := s.loadBook(a.Slug)
		if err != nil {
			return nil, nil, err
		}
		ch, err := findChapter(bc.Outline, a.Part, a.Chapter)
		if err != nil {
			return nil, nil, err
		}
		if a.Reviewer == "" {
			return nil, nil, fmt.Errorf("reviewer is required (who approved this chapter?)")
		}
		if ch.Status != book.StatusExpanded {
			return nil, nil, fmt.Errorf("chapter %02d-%02d status is %q; only expanded chapters can be reviewed", a.Part, a.Chapter, ch.Status)
		}
		now := time.Now().UTC()
		ch.Status = book.StatusReviewed
		ch.ReviewedAt = &now
		ch.ReviewedBy = a.Reviewer
		if err := book.SaveOutline(filepath.Join(bc.BookDir, "outline.json"), bc.Outline); err != nil {
			return nil, nil, err
		}
		if err := mirrorChapterStatus(bc.BookDir, a.Part, a.Chapter, book.StatusReviewed); err != nil {
			return nil, nil, err
		}
		return textResult("✓ reviewed %s %02d-%02d by %s（人工批准已记录）", a.Slug, a.Part, a.Chapter, a.Reviewer)
	})

	type finalizeArgs struct {
		Slug   string `json:"slug"`
		DryRun bool   `json:"dry_run,omitempty"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "finalize",
		Description: "Transition every reviewed chapter to final and set the book to final. Requires ALL chapters reviewed. Use dry_run to check blockers.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, a finalizeArgs) (*mcp.CallToolResult, any, error) {
		bc, err := s.loadBook(a.Slug)
		if err != nil {
			return nil, nil, err
		}
		var blockers []string
		for pi := range bc.Outline.Parts {
			for ci := range bc.Outline.Parts[pi].Chapters {
				c := &bc.Outline.Parts[pi].Chapters[ci]
				if c.Status != book.StatusReviewed && c.Status != book.StatusFinal {
					blockers = append(blockers, fmt.Sprintf("%02d-%02d %s（%s）", bc.Outline.Parts[pi].Index, c.Index, c.Title, c.Status))
				}
			}
		}
		if len(blockers) > 0 {
			return nil, nil, fmt.Errorf("cannot finalize: %d chapter(s) not reviewed: %s", len(blockers), strings.Join(blockers, "；"))
		}
		if a.DryRun {
			return textResult("dry-run: all chapters ready, finalize would succeed")
		}
		for pi := range bc.Outline.Parts {
			for ci := range bc.Outline.Parts[pi].Chapters {
				c := &bc.Outline.Parts[pi].Chapters[ci]
				if c.Status == book.StatusFinal {
					continue
				}
				c.Status = book.StatusFinal
				if err := mirrorChapterStatus(bc.BookDir, bc.Outline.Parts[pi].Index, c.Index, book.StatusFinal); err != nil {
					return nil, nil, err
				}
			}
		}
		bc.Meta.Status = book.BookStatusFinal
		bc.Meta.UpdatedAt = time.Now().UTC()
		if err := saveBookState(bc); err != nil {
			return nil, nil, err
		}
		return textResult("✓ finalized %s", a.Slug)
	})

	type exportArgs struct {
		Slug   string `json:"slug"`
		Target string `json:"target" jsonschema:"md | hugo | pdf | epub"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "export_book",
		Description: "Export the book (md/hugo/pdf/epub) as a background job. EPUB chapters carry source-verification sections. Returns job_id.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, a exportArgs) (*mcp.CallToolResult, any, error) {
		id := s.jobs.Start("export", a.Slug, func(ctx context.Context, j *Job) error {
			return s.runExport(ctx, j, a.Slug, a.Target)
		})
		return jobStartedResult(id, "export")
	})

	type publishArgs struct {
		Slug    string `json:"slug"`
		Version string `json:"version,omitempty" jsonschema:"explicit MAJOR.MINOR; default auto-derive"`
		Major   bool   `json:"major,omitempty" jsonschema:"force a major bump"`
		DryRun  bool   `json:"dry_run,omitempty" jsonschema:"report gate + next version without writing"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name: "publish",
		Description: "Publish an immutable versioned release (manifest + provenance + chapter snapshots + EPUB artifact) as a background job. " +
			"Hard gate: whole book final and meta.json license set — the license is the user's decision, never fill it for them. " +
			"Dry-run returns the gate report synchronously. Warnings (unverified claims etc.) are disclosed, not blocking.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, a publishArgs) (*mcp.CallToolResult, any, error) {
		bc, err := s.loadBook(a.Slug)
		if err != nil {
			return nil, nil, err
		}
		if a.DryRun {
			rep, stats := release.CheckGate(bc.Meta, bc.Outline)
			version, verr := release.NextVersion(storage.OS, filepath.Join(bc.BookDir, "releases"), bc.Outline,
				release.VersionOpts{Explicit: a.Version, ForceMajor: a.Major})
			if verr != nil {
				return nil, nil, verr
			}
			return jsonResult(map[string]any{"version": version, "gate": rep, "stats": stats})
		}
		id := s.jobs.Start("publish", a.Slug, func(ctx context.Context, j *Job) error {
			return s.runPublishJob(j, a.Slug, a.Version, a.Major)
		})
		return jobStartedResult(id, "publish")
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_releases",
		Description: "Published versions of one book (latest first) with manifest summaries.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, a bookArgs) (*mcp.CallToolResult, any, error) {
		bc, err := s.loadBook(a.Slug)
		if err != nil {
			return nil, nil, err
		}
		entries, _ := storage.OS.ReadDir(filepath.Join(bc.BookDir, "releases"))
		var out []map[string]any
		for _, e := range entries {
			if _, _, ok := release.ParseVersion(e.Name()); !ok {
				continue
			}
			raw, err := storage.OS.ReadFile(filepath.Join(bc.BookDir, "releases", e.Name(), "manifest.json"))
			if err != nil {
				continue
			}
			var m release.Manifest
			if json.Unmarshal(raw, &m) == nil {
				out = append(out, map[string]any{
					"version": m.Version, "created_at": m.CreatedAt, "chapters": m.Content.ChaptersTotal,
					"claims_unverified": m.Content.ClaimsUnverified, "has_epub": m.EPUB != nil,
				})
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i]["version"].(string) > out[j]["version"].(string) })
		return jsonResult(out)
	})

	type siteArgs struct {
		DryRun bool `json:"dry_run,omitempty"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "generate_site",
		Description: "Regenerate the static reading shelf (catalog, chapter pages, EPUB downloads, RSS + OPDS) from published releases only. Background job.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, a siteArgs) (*mcp.CallToolResult, any, error) {
		if a.DryRun {
			books, skipped, err := site.Scan(s.WSRoot())
			if err != nil {
				return nil, nil, err
			}
			return jsonResult(map[string]any{"books": len(books), "skipped": skipped})
		}
		id := s.jobs.Start("site", "", func(ctx context.Context, j *Job) error {
			return s.runSiteGenerate(j)
		})
		return jobStartedResult(id, "generate_site")
	})

	type jobArgs struct {
		ID string `json:"job_id"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "job_status",
		Description: "Poll a background job: status (pending/running/succeeded/failed), progress, log tail and result fields.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, a jobArgs) (*mcp.CallToolResult, any, error) {
		j, ok := s.jobs.Get(a.ID)
		if !ok {
			return nil, nil, fmt.Errorf("unknown job %q", a.ID)
		}
		log := j.Log
		if len(log) > 1500 {
			log = "…" + log[len(log)-1500:]
		}
		return jsonResult(map[string]any{
			"id": j.ID, "kind": j.Kind, "slug": j.Slug, "status": string(j.Status),
			"progress": j.Progress, "message": j.Message, "error": j.Err, "log_tail": log, "result": j.Result,
		})
	})
}

// jobStartedResult is the common "accepted" reply for async tools.
func jobStartedResult(id, kind string) (*mcp.CallToolResult, any, error) {
	return jsonResult(map[string]any{
		"job_id": id, "kind": kind,
		"next": "poll job_status with this job_id until status is succeeded or failed",
	})
}
