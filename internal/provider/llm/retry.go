package llm

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"
)

// RetryConfig controls retry behavior.
type RetryConfig struct {
	MaxAttempts int           // total attempts including the first; default 3
	BaseDelay   time.Duration // base for exponential backoff; default 1s
	MaxDelay    time.Duration // cap per delay; default 30s
	Jitter      float64       // ±fraction; default 0.2 (20%)
}

// DefaultRetryConfig matches Q7 decision: 3 attempts, 1s base, 30s cap, 20% jitter.
var DefaultRetryConfig = RetryConfig{
	MaxAttempts: 3,
	BaseDelay:   1 * time.Second,
	MaxDelay:    30 * time.Second,
	Jitter:      0.2,
}

// clock is a tiny indirection so tests can skip real Sleep.
type clock struct{}

func (clock) Sleep(d time.Duration) { time.Sleep(d) }

// RetryWrapper decorates a Chatter with retry on transient errors.
type RetryWrapper struct {
	Inner  Chatter
	Config RetryConfig
	clock  interface{ Sleep(time.Duration) } // injected for tests
}

// NewRetryWrapper constructs a RetryWrapper with default config and real clock.
func NewRetryWrapper(inner Chatter) *RetryWrapper {
	return &RetryWrapper{Inner: inner, Config: DefaultRetryConfig, clock: clock{}}
}

// shouldRetry reports whether the error is retryable per Q7.
func shouldRetry(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false // user cancelled or timed out — don't retry
	}
	if errors.Is(err, ErrNetwork) || errors.Is(err, ErrRateLimit) || errors.Is(err, ErrServer) {
		return true
	}
	return false // 4xx and unknown errors don't retry
}

func (rw *RetryWrapper) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	cfg := rw.Config
	if cfg.MaxAttempts == 0 {
		cfg = DefaultRetryConfig
	}
	clk := rw.clock
	if clk == nil {
		clk = clock{}
	}
	var lastErr error
	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		resp, err := rw.Inner.Chat(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !shouldRetry(err) {
			return nil, err
		}
		if attempt == cfg.MaxAttempts-1 {
			break // no more attempts
		}
		delay := backoff(cfg, attempt)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		clk.Sleep(delay)
	}
	return nil, fmt.Errorf("retry exhausted after %d attempts: %w", cfg.MaxAttempts, lastErr)
}

// backoff returns delay = BaseDelay * 2^attempt, capped at MaxDelay, with ±Jitter.
func backoff(cfg RetryConfig, attempt int) time.Duration {
	d := cfg.BaseDelay
	for i := 0; i < attempt && d < cfg.MaxDelay; i++ {
		d *= 2
	}
	if d > cfg.MaxDelay {
		d = cfg.MaxDelay
	}
	if cfg.Jitter > 0 {
		delta := float64(d) * cfg.Jitter
		d = time.Duration(float64(d) + (rand.Float64()*2-1)*delta)
	}
	return d
}
