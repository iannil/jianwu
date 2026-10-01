// Package deepseek adapts the shared OpenAI-compatible layer to the DeepSeek
// API (api.deepseek.com). DeepSeek rejects response_format json_schema, so
// JSONSchema requests are sent as json_object mode (schema stays
// prompt-driven), and the API offers no embeddings endpoint.
package deepseek

import (
	"github.com/iannil/jianwu/internal/provider/llm/openaicomp"
)

// DefaultBaseURL is the DeepSeek endpoint.
const DefaultBaseURL = "https://api.deepseek.com/v1"

// Config configures a DeepSeek provider.
type Config struct {
	APIKey  string
	BaseURL string // defaults to DefaultBaseURL if empty
}

// Provider implements llm.Chatter and llm.Streamer via DeepSeek's
// OpenAI-compatible API. Embed is not supported by the API.
type Provider = openaicomp.Provider

// New constructs a DeepSeek Provider.
func New(cfg Config) (*Provider, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	return openaicomp.New(openaicomp.Config{
		Name:          "deepseek",
		APIKey:        cfg.APIKey,
		BaseURL:       cfg.BaseURL,
		SchemaMode:    openaicomp.SchemaJSONObject,
		NoEmbeddings:  true,
	})
}
