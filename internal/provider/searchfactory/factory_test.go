package searchfactory

import (
	"testing"

	"github.com/zhurong/jianwu/internal/config"
)

func TestNewBrave(t *testing.T) {
	s, err := New("brave", &config.Secrets{BraveAPIKey: "k"})
	if err != nil {
		t.Fatal(err)
	}
	if s == nil {
		t.Fatal("nil")
	}
}

func TestNewSerper(t *testing.T) {
	s, err := New("serper", &config.Secrets{SerperAPIKey: "k"})
	if err != nil {
		t.Fatal(err)
	}
	if s == nil {
		t.Fatal("nil")
	}
}

func TestNewUnknownErrors(t *testing.T) {
	_, err := New("nope", &config.Secrets{})
	if err == nil {
		t.Error("expected error")
	}
}
