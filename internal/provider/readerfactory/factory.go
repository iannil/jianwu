package readerfactory

import (
	"fmt"

	"github.com/iannil/jianwu/internal/config"
	"github.com/iannil/jianwu/internal/provider/reader"
	"github.com/iannil/jianwu/internal/provider/reader/jina"
)

// New constructs a Reader by name. Names: "jina".
func New(name string, secrets *config.Secrets) (reader.Reader, error) {
	switch name {
	case "jina":
		return jina.New(jina.Config{APIKey: secrets.JinaAPIKey})
	default:
		return nil, fmt.Errorf("unknown reader provider: %q", name)
	}
}
