// Package glm adapts the shared OpenAI-compatible layer to GLM (智谱 BigModel).
package glm

import (
	"github.com/iannil/jianwu/internal/provider/llm/openaicomp"
)

// DefaultBaseURL is the GLM (智谱 BigModel) endpoint.
const DefaultBaseURL = "https://open.bigmodel.cn/api/paas/v4"

// Config configures a GLM provider.
type Config struct {
	APIKey  string
	BaseURL string // defaults to DefaultBaseURL if empty
}

// Provider implements llm.Chatter, llm.Embedder and llm.Streamer via GLM's
// OpenAI-compatible API.
type Provider = openaicomp.Provider

// New constructs a GLM Provider.
func New(cfg Config) (*Provider, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	return openaicomp.New(openaicomp.Config{
		Name:       "glm",
		APIKey:     cfg.APIKey,
		BaseURL:    cfg.BaseURL,
		SchemaMode: openaicomp.SchemaStrict,
	})
}
