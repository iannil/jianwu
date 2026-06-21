package llm

import (
	"errors"
	"fmt"
)

// Sentinel error categories. Used by Retry/Fallback wrappers to decide behavior.
var (
	// ErrNetwork = transient network failures (timeout, connection refused, DNS).
	// Triggers retry and fallback.
	ErrNetwork = errors.New("network error")
	// ErrLLMProvider = 4xx from the provider (auth, bad request, model not found).
	// Does NOT trigger retry or fallback (won't help).
	ErrLLMProvider = errors.New("llm provider error")
	// ErrRateLimit = 429. Triggers retry (with backoff) then fallback.
	ErrRateLimit = errors.New("rate limited")
	// ErrServer = 5xx from the provider. Triggers retry and fallback.
	ErrServer = errors.New("server error")
)

// HTTPError carries status code + body for diagnosis.
type HTTPError struct {
	Status int
	Body   string
	Inner  error // wrapped sentinel (ErrNetwork / ErrLLMProvider / etc.)
}

func (e *HTTPError) Error() string {
	if e.Inner != nil {
		return fmt.Sprintf("%s: HTTP %d: %s", e.Inner, e.Status, truncate(e.Body, 200))
	}
	return fmt.Sprintf("HTTP %d: %s", e.Status, truncate(e.Body, 200))
}

func (e *HTTPError) Unwrap() error { return e.Inner }

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// ClassifyError maps a raw error to one of the sentinel categories.
// Returns the sentinel (which the caller can errors.Is against) wrapped in a descriptive error.
func ClassifyError(err error, statusCode int) error {
	if err == nil {
		return nil
	}
	var sentinel error
	switch {
	case statusCode == 0:
		// No HTTP status: network/timeout/DNS.
		sentinel = ErrNetwork
	case statusCode == 429:
		sentinel = ErrRateLimit
	case statusCode >= 400 && statusCode < 500:
		sentinel = ErrLLMProvider
	case statusCode >= 500:
		sentinel = ErrServer
	default:
		sentinel = err
	}
	return &HTTPError{
		Status: statusCode,
		Body:   err.Error(),
		Inner:  sentinel,
	}
}
