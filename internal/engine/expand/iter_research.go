package expand

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/iannil/jianwu/internal/provider/llm"
)

// RunResearch executes iteration 1: query web_search for chapter-derived queries,
// read_url for top results, then ask LLM to extract research notes.
//
// If tools or WebSearchEnabled=false, skips tool calls and asks LLM with
// just the chapter context (fallback for offline mode).
func RunResearch(
	ctx context.Context,
	chatter llm.Chatter,
	tools *ToolRegistry,
	in ExpandInput,
) (ResearchNotes, error) {
	var notes ResearchNotes

	if in.WebSearchEnabled && tools != nil {
		// Plan: use chapter title + key concepts as queries.
		queries := buildResearchQueries(in)
		for _, q := range queries {
			results, err := tools.SearchAndRegister(ctx, q)
			if err != nil {
				// Tool limit reached or search error; try next query.
				continue
			}
			for i, r := range results {
				// Read top 2 results per query.
				if i >= 2 {
					break
				}
				if !looksLikeHTML(r.URL) {
					continue
				}
				_, _ = tools.ReadURL(ctx, r.URL) // best effort
			}
		}
	}

	// Build search results JSON for LLM context.
	var citedSnapshot []Citation
	if tools != nil {
		tools.mu.Lock()
		citedSnapshot = make([]Citation, 0, len(tools.citations))
		for _, c := range tools.citations {
			citedSnapshot = append(citedSnapshot, c)
		}
		tools.mu.Unlock()
	}
	searchJSON, _ := json.Marshal(citedSnapshot)

	sysBytes, _ := loadTemplate("system_research")
	userBytes, _ := loadTemplate("user_research")
	sys, err := renderExpand("system_research", sysBytes, map[string]any{})
	if err != nil {
		return notes, err
	}
	user, err := renderExpand("user_research", userBytes, map[string]any{
		"Topic":         in.Topic,
		"PartTitle":     in.PartTitle,
		"ChapterTitle":  in.ChapterTitle,
		"Abstract":      in.Abstract,
		"KeyConcepts":   strings.Join(in.KeyConcepts, ", "),
		"SearchResults": string(searchJSON),
	})
	if err != nil {
		return notes, err
	}

	schema, _ := JSONSchemaResearch()
	resp, err := chatter.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: sys},
			{Role: "user", Content: user},
		},
		JSONSchema: schema,
	})
	if err != nil {
		return notes, fmt.Errorf("research llm chat: %w", err)
	}
	if err := json.Unmarshal([]byte(resp.Content), &notes); err != nil {
		return notes, fmt.Errorf("parse research notes: %w (content: %s)", err, truncate(resp.Content, 500))
	}
	return notes, nil
}

// buildResearchQueries generates search queries from chapter context.
// Uses chapter title + each key concept.
func buildResearchQueries(in ExpandInput) []string {
	var queries []string
	if in.ChapterTitle != "" {
		queries = append(queries, in.Topic+" "+in.ChapterTitle)
	}
	for _, kc := range in.KeyConcepts {
		if len(queries) >= 3 {
			break
		}
		queries = append(queries, in.Topic+" "+kc)
	}
	return queries
}

func looksLikeHTML(url string) bool {
	// Simple filter: skip PDF, images, etc. for v1.
	lower := strings.ToLower(url)
	if strings.HasSuffix(lower, ".pdf") || strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".png") {
		return false
	}
	return true
}

func renderExpand(name string, raw []byte, data any) (string, error) {
	tmpl, err := template.New(name).Parse(string(raw))
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", name, err)
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute %s: %w", name, err)
	}
	return buf.String(), nil
}
