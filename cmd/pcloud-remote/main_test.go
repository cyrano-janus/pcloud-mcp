package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthAndFailClosedMCP(t *testing.T) {
	h := handler()
	for _, tt := range []struct {
		method, path string
		want         int
	}{
		{"GET", "/healthz", http.StatusOK},
		{"POST", "/mcp", http.StatusServiceUnavailable},
		{"GET", "/mcp", http.StatusServiceUnavailable},
		{"DELETE", "/mcp", http.StatusServiceUnavailable},
		{"GET", "/unknown", http.StatusNotFound},
	} {
		req := httptest.NewRequest(tt.method, tt.path, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != tt.want {
			t.Errorf("%s %s = %d, want %d", tt.method, tt.path, rr.Code, tt.want)
		}
		if tt.path == "/mcp" {
			if rr.Header().Get("Cache-Control") != "no-store" {
				t.Error("MCP error must not be cached")
			}
			b, _ := io.ReadAll(rr.Body)
			if len(b) == 0 {
				t.Error("expected disabled response")
			}
		}
	}
}

func TestInvalidPortRejected(t *testing.T) {
	for _, port := range []string{"1234:4321", "-1", "100000", "xxx", "8080/evil"} {
		if err := run(context.Background(), port); err == nil {
			t.Errorf("accepted invalid port %q", port)
		}
	}
}

func TestHostedSetupDisabledByDefault(t *testing.T) {
	t.Setenv("PCLOUD_SETUP_ENABLED", "")
	setup, err := setupFromEnv()
	if err != nil || setup != nil {
		t.Fatal("setup enabled without configuration")
	}
	for _, path := range []string{"/setup/pcloud", "/oauth/pcloud/callback"} {
		r := httptest.NewRecorder()
		handler().ServeHTTP(r, httptest.NewRequest("GET", path, nil))
		if r.Code != 503 {
			t.Fatal("setup publicly available", path, r.Code)
		}
	}
	t.Setenv("PCLOUD_SETUP_ENABLED", "true")
	t.Setenv("RENDER", "")
	if _, err := setupFromEnv(); err == nil {
		t.Fatal("untrusted host accepted")
	}
	t.Setenv("RENDER", "true")
	t.Setenv("RENDER_EXTERNAL_URL", "https://service.example")
	t.Setenv("PCLOUD_CLIENT_ID", "")
	if _, err := setupFromEnv(); err == nil {
		t.Fatal("incomplete setup enabled")
	}
}
