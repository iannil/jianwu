package search

import "errors"

// ErrProvider is returned for 4xx responses from a search API.
// Does NOT trigger retry (won't help).
var ErrProvider = errors.New("search provider error")

// ErrRateLimit is returned for 429. Triggers retry/fallback.
var ErrRateLimit = errors.New("search rate limited")

// ErrNetwork is returned for network/timeout errors. Triggers retry/fallback.
var ErrNetwork = errors.New("search network error")
