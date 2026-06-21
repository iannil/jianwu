package gemini

import (
	"context"
	"os"
	"testing"

	"github.com/zhurong/jianwu/internal/provider/llm"
)

// Tests against the real SDK would require a live API key, so we test only
// the construction + config plumbing. Real API behavior is covered by
// integration tests that skip when GEMINI_API_KEY is unset.
//
// The Provider.Chat / Provider.Embed methods will be exercised in integration
// tests run manually before S2 release.

func TestNewRequiresAPIKey(t *testing.T) {
	_, err := New(Config{APIKey: ""})
	if err == nil {
		t.Error("expected error on empty API key")
	}
}

func TestNewConstructsWithKey(t *testing.T) {
	// We don't actually call the SDK here; we just verify construction.
	// Real client init happens lazily on first call (or we accept the offline init).
	p, err := New(Config{APIKey: "fake-key"})
	if err != nil {
		t.Fatal(err)
	}
	if p == nil {
		t.Fatal("nil provider")
	}
}

// TestProviderChatWithLiveKey is an integration test that runs only when
// GEMINI_API_KEY is set. Skip otherwise.
func TestProviderChatWithLiveKey(t *testing.T) {
	key := apiKeyFromEnv()
	if key == "" {
		t.Skip("GEMINI_API_KEY not set; skipping live test")
	}
	p, err := New(Config{APIKey: key})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := p.Chat(context.Background(), llm.ChatRequest{
		Model:    "gemini-2.5-flash",
		Messages: []llm.Message{{Role: "user", Content: "Say 'hello' and nothing else."}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Content == "" {
		t.Error("empty content")
	}
	t.Logf("response: %q (tokens in=%d out=%d)", resp.Content, resp.TokensIn, resp.TokensOut)
}

// apiKeyFromEnv reads the Gemini API key from environment (for tests).
func apiKeyFromEnv() string {
	return os.Getenv("GEMINI_API_KEY")
}
