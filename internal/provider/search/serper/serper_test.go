package serper

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
		if r.Header.Get("X-API-KEY") != "test-key" {
			t.Errorf("key: %q", r.Header.Get("X-API-KEY"))
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["q"] != "hello" {
			t.Errorf("q: %v", body["q"])
		}
		json.NewEncoder(w).Encode(map[string]any{
			"organic": []map[string]any{
				{"title": "R1", "link": "https://example.com/1", "snippet": "First"},
			},
		})
	}))
	defer srv.Close()
	s, _ := New(Config{APIKey: "test-key", BaseURL: srv.URL})
	results, err := s.Search(context.Background(), "hello", search.SearchOpts{MaxResults: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d", len(results))
	}
	if results[0].Title != "R1" {
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
	if !errors.Is(err, search.ErrProvider) {
		t.Errorf("expected ErrProvider, got %v", err)
	}
}
