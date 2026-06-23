package readerfactory

import (
	"testing"

	"github.com/iannil/jianwu/internal/config"
)

func TestNewJina(t *testing.T) {
	r, err := New("jina", &config.Secrets{JinaAPIKey: "k"})
	if err != nil {
		t.Fatal(err)
	}
	if r == nil {
		t.Fatal("nil")
	}
}

func TestNewUnknownErrors(t *testing.T) {
	_, err := New("nope", &config.Secrets{})
	if err == nil {
		t.Error("expected error")
	}
}
