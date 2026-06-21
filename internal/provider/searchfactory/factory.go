package searchfactory

import (
	"fmt"

	"github.com/zhurong/jianwu/internal/config"
	"github.com/zhurong/jianwu/internal/provider/search"
	"github.com/zhurong/jianwu/internal/provider/search/brave"
	"github.com/zhurong/jianwu/internal/provider/search/serper"
)

// New constructs a Searcher by name. Names: "brave", "serper".
func New(name string, secrets *config.Secrets) (search.Searcher, error) {
	switch name {
	case "brave":
		if secrets.BraveAPIKey == "" {
			return nil, fmt.Errorf("brave requires BRAVE_API_KEY")
		}
		return brave.New(brave.Config{APIKey: secrets.BraveAPIKey})
	case "serper":
		if secrets.SerperAPIKey == "" {
			return nil, fmt.Errorf("serper requires SERPER_API_KEY")
		}
		return serper.New(serper.Config{APIKey: secrets.SerperAPIKey})
	default:
		return nil, fmt.Errorf("unknown search provider: %q", name)
	}
}
