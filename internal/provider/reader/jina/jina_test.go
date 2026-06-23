package jina

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iannil/jianwu/internal/provider/reader"
)

func TestReadSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth: %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Title: Example\n\nThis is the cleaned content."))
	}))
	defer srv.Close()
	rdr, err := New(Config{APIKey: "test-key", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	content, err := rdr.Read(context.Background(), "https://example.com/foo")
	if err != nil {
		t.Fatal(err)
	}
	if content.Markdown == "" {
		t.Error("empty markdown")
	}
}

func TestRead4xxReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	rdr, _ := New(Config{APIKey: "k", BaseURL: srv.URL})
	_, err := rdr.Read(context.Background(), "https://example.com/missing")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, reader.ErrReader) {
		t.Errorf("expected ErrReader, got %v", err)
	}
}
