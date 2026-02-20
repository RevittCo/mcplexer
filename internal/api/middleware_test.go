package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeadersMiddleware(t *testing.T) {
	h := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://localhost/api/v1/health", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("expected nosniff header, got %q", got)
	}
	if got := rr.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("expected DENY frame header, got %q", got)
	}
	if got := rr.Header().Get("Content-Security-Policy"); got == "" {
		t.Fatal("expected CSP header to be set")
	}
}

func TestBrowserOriginProtectionMiddleware(t *testing.T) {
	h := browserOriginProtectionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	t.Run("allows localhost origin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://localhost/api/v1/workspaces", nil)
		req.Header.Set("Origin", "http://localhost:5173")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("expected %d, got %d", http.StatusNoContent, rr.Code)
		}
	})

	t.Run("blocks non-local origin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://localhost/api/v1/workspaces", nil)
		req.Header.Set("Origin", "https://evil.example")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected %d, got %d", http.StatusForbidden, rr.Code)
		}
	})

	t.Run("blocks cross-site fetch hint", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://localhost/api/v1/workspaces", nil)
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected %d, got %d", http.StatusForbidden, rr.Code)
		}
	})
}

func TestRequireJSONContentTypeMiddleware(t *testing.T) {
	h := requireJSONContentTypeMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	t.Run("rejects non-json content type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://localhost/api/v1/workspaces", strings.NewReader(`{"name":"x"}`))
		req.Header.Set("Content-Type", "text/plain")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnsupportedMediaType {
			t.Fatalf("expected %d, got %d", http.StatusUnsupportedMediaType, rr.Code)
		}
	})

	t.Run("allows json content type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://localhost/api/v1/workspaces", strings.NewReader(`{"name":"x"}`))
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("expected %d, got %d", http.StatusNoContent, rr.Code)
		}
	})

	t.Run("allows empty post body without content type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://localhost/api/v1/cache/flush", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("expected %d, got %d", http.StatusNoContent, rr.Code)
		}
	})
}

func TestCORSMiddleware(t *testing.T) {
	h := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	t.Run("local origin gets cors headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://localhost/api/v1/workspaces", nil)
		req.Header.Set("Origin", "http://localhost:5173")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
			t.Fatalf("unexpected allow-origin header: %q", got)
		}
	})

	t.Run("blocks non-local preflight", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "http://localhost/api/v1/workspaces", nil)
		req.Header.Set("Origin", "https://evil.example")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected %d, got %d", http.StatusForbidden, rr.Code)
		}
	})
}
