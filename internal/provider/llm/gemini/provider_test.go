package gemini

import (
	"context"
	"os"
	"testing"

	"github.com/zhurong/jianwu/internal/provider/llm"
	"google.golang.org/genai"
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

// TestSchemaFromRawTranslatesBasicObject verifies that schemaFromRaw
// correctly translates a simple JSON Schema into genai.Schema.
func TestSchemaFromRawTranslatesBasicObject(t *testing.T) {
	raw := []byte(`{
		"type": "object",
		"properties": {
			"foo": {"type": "string"},
			"bar": {"type": "number"}
		},
		"required": ["foo"]
	}`)
	s := &genai.Schema{}
	if err := schemaFromRaw(raw, s); err != nil {
		t.Fatalf("schemaFromRaw: %v", err)
	}
	if s.Type != "OBJECT" {
		t.Errorf("type: got %q, want OBJECT", s.Type)
	}
	if len(s.Properties) != 2 {
		t.Fatalf("properties count: got %d, want 2", len(s.Properties))
	}
	if s.Properties["foo"].Type != "STRING" {
		t.Errorf("foo type: got %q, want STRING", s.Properties["foo"].Type)
	}
	if s.Properties["bar"].Type != "NUMBER" {
		t.Errorf("bar type: got %q, want NUMBER", s.Properties["bar"].Type)
	}
	if len(s.Required) != 1 || s.Required[0] != "foo" {
		t.Errorf("required: got %v, want [foo]", s.Required)
	}
}
