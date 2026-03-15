package addon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
)

// maxResponseBytes is the maximum response body size before truncation.
const maxResponseBytes = 200 * 1024 // 200KB

// placeholderRe matches {{param}} placeholders in URLs and query params.
var placeholderRe = regexp.MustCompile(`\{\{(\w+)\}\}`)

// AuthHeaderFunc returns auth headers for the given auth scope (server ID).
type AuthHeaderFunc func(ctx context.Context, authScopeID string) (http.Header, error)

// Executor makes HTTP requests for addon tools.
type Executor struct {
	getAuthHeaders AuthHeaderFunc
	client         *http.Client
}

// NewExecutor creates an Executor that uses authFn to obtain OAuth headers.
func NewExecutor(authFn AuthHeaderFunc) *Executor {
	return &Executor{
		getAuthHeaders: authFn,
		client:         &http.Client{},
	}
}

// callToolResult mirrors the MCP CallToolResult structure.
type callToolResult struct {
	Content []toolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Execute runs an addon tool call by making the configured HTTP request.
func (e *Executor) Execute(
	ctx context.Context,
	tool *ResolvedTool,
	authScopeID string,
	args json.RawMessage,
) (json.RawMessage, error) {
	// Parse arguments.
	var params map[string]any
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, fmt.Errorf("unmarshal args: %w", err)
		}
	}
	if params == nil {
		params = make(map[string]any)
	}

	consumed := make(map[string]bool)

	// Build URL with placeholder substitution.
	url, err := substituteURL(tool.URL, params, consumed)
	if err != nil {
		return nil, fmt.Errorf("build url: %w", err)
	}

	// Build query string.
	url = appendQueryParams(url, tool.QueryParams, params, consumed)

	// Build request body for methods that support it.
	var bodyReader io.Reader
	method := strings.ToUpper(tool.Method)
	if methodHasBody(method) && tool.BodyMapping != "none" {
		body := buildBody(params, consumed)
		if body != nil {
			encoded, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("marshal body: %w", err)
			}
			bodyReader = bytes.NewReader(encoded)
		}
	}

	// Create HTTP request.
	httpReq, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if bodyReader != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	// Apply static headers from the tool definition.
	for k, v := range tool.Headers {
		httpReq.Header.Set(k, v)
	}

	// Inject OAuth headers from the parent server.
	authHeaders, err := e.getAuthHeaders(ctx, authScopeID)
	if err != nil {
		return nil, fmt.Errorf("get auth headers: %w", err)
	}
	for k, vals := range authHeaders {
		for _, v := range vals {
			httpReq.Header.Set(k, v)
		}
	}

	// Execute request.
	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body with truncation.
	respBody, err := readTruncated(resp.Body, maxResponseBytes)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	slog.Debug("addon tool executed",
		"tool", tool.FullName,
		"status", resp.StatusCode,
		"body_len", len(respBody),
	)

	// Build MCP-format result.
	result := callToolResult{
		Content: []toolContent{{
			Type: "text",
			Text: string(respBody),
		}},
	}

	if resp.StatusCode >= 400 {
		result.IsError = true
		result.Content[0].Text = fmt.Sprintf(
			"HTTP %d: %s", resp.StatusCode, respBody,
		)
	}

	return json.Marshal(result)
}

// substituteURL replaces {{param}} placeholders in the URL template.
func substituteURL(tmpl string, params map[string]any, consumed map[string]bool) (string, error) {
	var lastErr error
	result := placeholderRe.ReplaceAllStringFunc(tmpl, func(match string) string {
		name := placeholderRe.FindStringSubmatch(match)[1]
		val, ok := params[name]
		if !ok {
			lastErr = fmt.Errorf("missing required url param %q", name)
			return match
		}
		consumed[name] = true
		return fmt.Sprintf("%v", val)
	})
	if lastErr != nil {
		return "", lastErr
	}
	return result, nil
}

// appendQueryParams adds query parameters to the URL, substituting
// {{param}} references. Missing optional params are silently skipped.
func appendQueryParams(
	url string,
	queryDefs map[string]string,
	params map[string]any,
	consumed map[string]bool,
) string {
	if len(queryDefs) == 0 {
		return url
	}

	sep := "?"
	if strings.Contains(url, "?") {
		sep = "&"
	}

	var parts []string
	for key, valTmpl := range queryDefs {
		resolved := placeholderRe.ReplaceAllStringFunc(valTmpl, func(match string) string {
			name := placeholderRe.FindStringSubmatch(match)[1]
			val, ok := params[name]
			if !ok {
				return ""
			}
			consumed[name] = true
			return fmt.Sprintf("%v", val)
		})
		if resolved == "" {
			continue // skip empty/missing optional params
		}
		parts = append(parts, key+"="+resolved)
	}

	if len(parts) == 0 {
		return url
	}
	return url + sep + strings.Join(parts, "&")
}

// buildBody creates a request body from arguments not consumed by URL or query params.
func buildBody(params map[string]any, consumed map[string]bool) map[string]any {
	body := make(map[string]any)
	for k, v := range params {
		if !consumed[k] {
			body[k] = v
		}
	}
	if len(body) == 0 {
		return nil
	}
	return body
}

// methodHasBody returns true for HTTP methods that typically carry a body.
func methodHasBody(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return true
	}
	return false
}

// readTruncated reads up to maxBytes from r, appending a truncation notice
// if the response exceeds the limit.
func readTruncated(r io.Reader, maxBytes int) ([]byte, error) {
	limited := io.LimitReader(r, int64(maxBytes+1))
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(data) > maxBytes {
		data = data[:maxBytes]
		data = append(data, "\n... [truncated at 200KB]"...)
	}
	return data, nil
}
