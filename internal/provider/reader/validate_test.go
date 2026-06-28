package reader

import (
	"errors"
	"testing"
)

func TestValidateURLAllowsPublicHTTPS(t *testing.T) {
	if err := ValidateURL("https://example.com/article"); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if err := ValidateURL("https://zhurongshuo.com/books/reality-construction/"); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateURLRejectsLocalhost(t *testing.T) {
	if err := ValidateURL("http://localhost:8080/secret"); !errors.Is(err, ErrSSRFBlocked) {
		t.Errorf("expected ErrSSRFBlocked, got: %v", err)
	}
	if err := ValidateURL("http://127.0.0.1/admin"); !errors.Is(err, ErrSSRFBlocked) {
		t.Errorf("expected ErrSSRFBlocked, got: %v", err)
	}
}

func TestValidateURLRejectsNonHTTPScheme(t *testing.T) {
	if err := ValidateURL("ftp://files.example.com/data"); !errors.Is(err, ErrSSRFBlocked) {
		t.Errorf("expected ErrSSRFBlocked, got: %v", err)
	}
	if err := ValidateURL("file:///etc/passwd"); !errors.Is(err, ErrSSRFBlocked) {
		t.Errorf("expected ErrSSRFBlocked, got: %v", err)
	}
}

func TestValidateURLRejectsInternalHostnames(t *testing.T) {
	if err := ValidateURL("http://internal-service.local/config"); !errors.Is(err, ErrSSRFBlocked) {
		t.Errorf("expected ErrSSRFBlocked, got: %v", err)
	}
	if err := ValidateURL("https://db.internal/query"); !errors.Is(err, ErrSSRFBlocked) {
		t.Errorf("expected ErrSSRFBlocked, got: %v", err)
	}
}

func TestValidateURLRejectsPrivateIP(t *testing.T) {
	// 10.x.x.x is private
	if err := ValidateURL("http://10.0.0.1/admin"); !errors.Is(err, ErrSSRFBlocked) {
		t.Errorf("expected ErrSSRFBlocked for 10.x, got: %v", err)
	}
	// 192.168.x.x is private
	if err := ValidateURL("http://192.168.1.1/config"); !errors.Is(err, ErrSSRFBlocked) {
		t.Errorf("expected ErrSSRFBlocked for 192.168.x, got: %v", err)
	}
}

func TestValidateURLHandlesBadURL(t *testing.T) {
	if err := ValidateURL("not-a-url"); err == nil {
		t.Error("expected error for invalid URL")
	}
}
