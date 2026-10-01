// Package kimi adapts the shared OpenAI-compatible layer to the Kimi API
// (Moonshot AI, platform.kimi.ai). Kimi K-series models (e.g. kimi-k3)
// support strict json_schema structured output.
package kimi

import (
	"github.com/iannil/jianwu/internal/provider/llm/openaicomp"
)

// DefaultBaseURL is the Kimi (Moonshot AI) endpoint.
const DefaultBaseURL = "https://api.moonshot.ai/v1"

// Config configures a Kimi provider.
type Config struct {
	APIKey  string
	BaseURL string // defaults to DefaultBaseURL if empty
}

// Provider implements llm.Chatter, llm.Embedder and llm.Streamer via Kimi's
// OpenAI-compatible API.
type Provider = openaicomp.Provider

// New constructs a Kimi Provider.
func New(cfg Config) (*Provider, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	return openaicomp.New(openaicomp.Config{
		Name:       "kimi",
		APIKey:     cfg.APIKey,
		BaseURL:    cfg.BaseURL,
		SchemaMode: openaicomp.SchemaStrict,
	})
}
