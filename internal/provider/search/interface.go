package search

import (
	"context"
	"time"
)

// Searcher is the web-search interface. Implementations: brave.Searcher, serper.Searcher.
type Searcher interface {
	Search(ctx context.Context, query string, opts SearchOpts) ([]SearchResult, error)
}

// SearchOpts controls a single search query.
type SearchOpts struct {
	MaxResults int           // default 10
	TimeRange  TimeRange     // "any" (default) | "past_day" | "past_week" | "past_month" | "past_year"
	Language   string        // BCP-47 like "zh-CN", "en-US"; empty = no filter
}

// TimeRange is an enum for SearchOpts.TimeRange.
type TimeRange string

const (
	TimeAny   TimeRange = "any"
	TimeDay   TimeRange = "past_day"
	TimeWeek  TimeRange = "past_week"
	TimeMonth TimeRange = "past_month"
	TimeYear  TimeRange = "past_year"
)

// SearchResult is one hit from a search query.
type SearchResult struct {
	Title   string
	URL     string
	Snippet string
	// PublishedAt is set when the provider returns it (Brave does; Serper doesn't).
	PublishedAt *time.Time
}
