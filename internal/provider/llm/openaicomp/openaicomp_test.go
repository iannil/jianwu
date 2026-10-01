package openaicomp

import (
	"strings"
	"testing"
)

func TestNewValidation(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{"valid", Config{Name: "x", APIKey: "k", BaseURL: "http://api"}, ""},
		{"missing name", Config{APIKey: "k", BaseURL: "http://api"}, "Name is required"},
		{"missing key", Config{Name: "x", BaseURL: "http://api"}, "x: APIKey is required"},
		{"missing base url", Config{Name: "x", APIKey: "k"}, "x: BaseURL is required"},
	}
	for _, tc := range cases {
		p, err := New(tc.cfg)
		if tc.wantErr == "" {
			if err != nil {
				t.Errorf("%s: unexpected error %v", tc.name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
			t.Errorf("%s: want error containing %q, got %v", tc.name, tc.wantErr, err)
		}
		if p != nil {
			t.Errorf("%s: want nil provider on error", tc.name)
		}
	}
}

func TestNewDefaultsSchemaMode(t *testing.T) {
	p, err := New(Config{Name: "x", APIKey: "k", BaseURL: "http://api"})
	if err != nil {
		t.Fatal(err)
	}
	if p.cfg.SchemaMode != SchemaStrict {
		t.Errorf("SchemaMode default = %q, want %q", p.cfg.SchemaMode, SchemaStrict)
	}
}
