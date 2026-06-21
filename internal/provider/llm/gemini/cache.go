package gemini

import (
	"context"

	"google.golang.org/genai"
)

// CreateCache creates a context cache for a shared system prompt.
// Returns the cache name to use in subsequent GenerateContent calls.
//
// Usage (when wired into engine in S3+):
//
//	cacheName, _ := gemini.CreateCache(ctx, client, "gemini-2.5-pro", systemPrompt)
//	config.CachedContent = cacheName
//	resp, _ := client.Models.GenerateContent(ctx, model, contents, config)
//
// For S2 this is a helper that downstream tasks can use; it's not yet wired
// into Chat because we don't know the cache granularity (per-stage? per-book?).
func CreateCache(ctx context.Context, client *genai.Client, model, systemPrompt string) (string, error) {
	resp, err := client.Caches.Create(ctx, model, &genai.CreateCachedContentConfig{
		Contents: []*genai.Content{
			genai.NewContentFromText(systemPrompt, genai.RoleUser),
		},
	})
	if err != nil {
		return "", err
	}
	return resp.Name, nil
}
