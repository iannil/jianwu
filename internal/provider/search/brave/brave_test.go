package brave

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iannil/jianwu/internal/provider/search"
)

func TestSearchSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Subscription-Token") != "test-key" {
			t.Errorf("token: %q", r.Header.Get("X-Subscription-Token"))
		}
		if r.URL.Query().Get("q") != "hello world" {
			t.Errorf("q: %q", r.URL.Query().Get("q"))
		}
		if r.URL.Query().Get("count") != "5" {
			t.Errorf("count: %q", r.URL.Query().Get("count"))
		}
		json.NewEncoder(w).Encode(map[string]any{
			"web": map[string]any{
				"results": []map[string]any{
					{"title": "Result 1", "url": "https://example.com/1", "description": "First"},
					{"title": "Result 2", "url": "https://example.com/2", "description": "Second"},
				},
			},
		})
	}))
	defer srv.Close()

	s, err := New(Config{APIKey: "test-key", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	results, err := s.Search(context.Background(), "hello world", search.SearchOpts{MaxResults: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results", len(results))
	}
	if results[0].Title != "Result 1" {
		t.Errorf("title: %q", results[0].Title)
	}
}

func TestSearch4xxReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"message": "invalid token"})
	}))
	defer srv.Close()

	s, _ := New(Config{APIKey: "bad", BaseURL: srv.URL})
	_, err := s.Search(context.Background(), "x", search.SearchOpts{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, search.ErrSearchProvider) {
		t.Errorf("expected ErrSearchProvider, got %v", err)
	}
}
