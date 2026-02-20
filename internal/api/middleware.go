package api

import (
	"context"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

type contextKey string

const requestIDKey contextKey = "request_id"

const (
	maxRequestBodyBytes = int64(1 << 20) // 1 MiB
	defaultCSP          = "default-src 'self'; " +
		"base-uri 'self'; " +
		"frame-ancestors 'none'; " +
		"object-src 'none'; " +
		"script-src 'self'; " +
		"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; " +
		"font-src 'self' data: https://fonts.gstatic.com; " +
		"img-src 'self' data:; " +
		"connect-src 'self'; " +
		"form-action 'self'"
)

// requestIDMiddleware injects a unique request ID into the request context
// and sets it as a response header.
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.New().String()
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// loggingMiddleware logs each request with method, path, status, and duration.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", r.Context().Value(requestIDKey),
		)
	})
}

// corsMiddleware allows requests from localhost origins for development.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if isLocalOrigin(origin) {
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
			w.Header().Set("Access-Control-Max-Age", "3600")
		}
		if r.Method == http.MethodOptions {
			if origin != "" && !isLocalOrigin(origin) {
				writeError(w, http.StatusForbidden, "cross-origin browser request denied")
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// securityHeadersMiddleware applies hardened browser response headers.
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", defaultCSP)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// browserOriginProtectionMiddleware blocks browser requests from non-local origins.
// This mitigates localhost CSRF and DNS rebinding abuse against unauthenticated local APIs.
func browserOriginProtectionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && !isLocalOrigin(origin) {
			writeError(w, http.StatusForbidden, "cross-origin browser request denied")
			return
		}

		if origin == "" {
			site := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")))
			if site == "cross-site" {
				writeError(w, http.StatusForbidden, "cross-origin browser request denied")
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// requestBodyLimitMiddleware applies a global max body size for request handlers.
func requestBodyLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hasRequestBody(r) {
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// requireJSONContentTypeMiddleware enforces application/json for mutating requests
// that include a request body.
func requireJSONContentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !expectsJSONBody(r.Method) || !hasRequestBody(r) {
			next.ServeHTTP(w, r)
			return
		}

		contentType := strings.TrimSpace(r.Header.Get("Content-Type"))
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || mediaType != "application/json" {
			writeError(w, http.StatusUnsupportedMediaType, "content-type must be application/json")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func expectsJSONBody(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return true
	default:
		return false
	}
}

func hasRequestBody(r *http.Request) bool {
	if r == nil || r.Body == nil || r.Body == http.NoBody {
		return false
	}
	if r.ContentLength > 0 {
		return true
	}
	return strings.TrimSpace(r.Header.Get("Transfer-Encoding")) != ""
}

// isLocalOrigin returns true for localhost/127.0.0.1 origins.
func isLocalOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// statusWriter captures the HTTP status code for logging.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Flush delegates to the underlying ResponseWriter so SSE handlers work.
func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
