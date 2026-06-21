package search

import "errors"

// ErrSearchProvider is returned for 4xx responses from a search API.
// Does NOT trigger retry (won't help).
var ErrSearchProvider = errors.New("search provider error")

// ErrSearchRateLimit is returned for 429. Triggers retry/fallback.
var ErrSearchRateLimit = errors.New("search rate limited")

// ErrSearchNetwork is returned for network/timeout errors. Triggers retry/fallback.
var ErrSearchNetwork = errors.New("search network error")
