package downstream

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ErrAuthRequired indicates the downstream server returned 401 and needs OAuth.
var ErrAuthRequired = errors.New("downstream server requires authentication")

// HTTPInstance communicates with a remote MCP server over Streamable HTTP
// (JSON-RPC over HTTP POST). Each request is a separate HTTP POST.
type HTTPInstance struct {
	key    InstanceKey
	url    string
	client *http.Client

	mu          sync.Mutex
	state       InstanceState
	authHeaders http.Header
	sessionID   string // Mcp-Session-Id from server

	idleTimeout time.Duration
	idleTimer   *time.Timer
	reqID       atomic.Int64

	sessionURL string // may be updated by server via Location header
}

func newHTTPInstance(key InstanceKey, url string, idleTimeout time.Duration, headers http.Header) *HTTPInstance {
	return &HTTPInstance{
		key:         key,
		url:         url,
		state:       StateStopped,
		authHeaders: headers,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		idleTimeout: idleTimeout,
	}
}

// SetAuthHeaders updates the authorization headers injected on every request.
func (h *HTTPInstance) SetAuthHeaders(headers http.Header) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.authHeaders = headers
}

func (h *HTTPInstance) getState() InstanceState {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.state
}

func (h *HTTPInstance) start(ctx context.Context) error {
	h.mu.Lock()
	if h.state != StateStopped {
		s := h.state
		h.mu.Unlock()
		return fmt.Errorf("cannot start http instance in state %s", s)
	}
	h.state = StateStarting
	h.mu.Unlock()

	// Perform MCP initialize handshake over HTTP (mutex released so doRPC can read authHeaders).
	initReq := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "initialize",
		Params: json.RawMessage(`{
			"protocolVersion": "2024-11-05",
			"capabilities": {},
			"clientInfo": {"name": "mcplexer", "version": "0.1.0"}
		}`),
	}

	resp, err := h.doRPC(ctx, initReq)
	if err != nil {
		h.mu.Lock()
		h.state = StateStopped
		h.mu.Unlock()
		return fmt.Errorf("initialize: %w", err)
	}
	_ = resp // initialize response not needed

	// Send initialized notification (no ID = notification).
	notif := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	if _, err := h.doRPC(ctx, notif); err != nil {
		// Non-fatal: some servers don't handle this
	}

	h.mu.Lock()
	h.state = StateReady
	h.mu.Unlock()
	return nil
}

func (h *HTTPInstance) stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.idleTimer != nil {
		h.idleTimer.Stop()
	}
	h.state = StateStopped
}

// ListTools sends a tools/list request to the HTTP MCP server.
func (h *HTTPInstance) ListTools(ctx context.Context) (json.RawMessage, error) {
	h.mu.Lock()
	h.state = StateBusy
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		h.state = StateIdle
		h.resetIdleTimer()
		h.mu.Unlock()
	}()

	id := h.reqID.Add(1)
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(fmt.Sprintf(`%d`, id)),
		Method:  "tools/list",
		Params:  json.RawMessage(`{}`),
	}

	return h.doRPC(ctx, req)
}

// Call sends a tools/call request to the HTTP MCP server.
func (h *HTTPInstance) Call(
	ctx context.Context, method string, params json.RawMessage,
) (json.RawMessage, error) {
	h.mu.Lock()
	h.state = StateBusy
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		h.state = StateIdle
		h.resetIdleTimer()
		h.mu.Unlock()
	}()

	id := h.reqID.Add(1)
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(fmt.Sprintf(`%d`, id)),
		Method:  method,
		Params:  params,
	}

	return h.doRPC(ctx, req)
}

// doRPC sends a JSON-RPC request via HTTP POST and returns the result.
func (h *HTTPInstance) doRPC(ctx context.Context, rpcReq jsonRPCRequest) (json.RawMessage, error) {
	body, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := h.url
	if h.sessionURL != "" {
		url = h.sessionURL
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")

	// Inject auth headers (e.g. Authorization: Bearer <token>).
	h.mu.Lock()
	headers := h.authHeaders
	sid := h.sessionID
	h.mu.Unlock()
	for k, vals := range headers {
		for _, v := range vals {
			httpReq.Header.Set(k, v)
		}
	}

	// Include session ID from previous initialize handshake.
	if sid != "" {
		httpReq.Header.Set("Mcp-Session-Id", sid)
	}

	resp, err := h.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	// Capture session ID from server (returned on initialize, echoed thereafter).
	if v := resp.Header.Get("Mcp-Session-Id"); v != "" {
		h.mu.Lock()
		h.sessionID = v
		h.mu.Unlock()
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrAuthRequired
	}

	// Notifications return 202 with no body.
	if rpcReq.ID == nil {
		if resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusOK {
			return nil, nil
		}
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("notification failed (%d): %s", resp.StatusCode, respBody)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, respBody)
	}

	ct := resp.Header.Get("Content-Type")

	// Handle SSE responses (text/event-stream).
	if strings.HasPrefix(ct, "text/event-stream") {
		return h.readSSEResponse(resp.Body)
	}

	// Standard JSON response.
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

// readSSEResponse reads a text/event-stream response and extracts the JSON-RPC result.
// Per MCP Streamable HTTP spec, the server sends SSE events with "data:" lines.
func (h *HTTPInstance) readSSEResponse(body io.Reader) (json.RawMessage, error) {
	scanner := bufio.NewScanner(body)
	// GitHub's MCP API returns large tool lists that exceed the default 64KB buffer.
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024) // up to 4MB
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		var rpcResp jsonRPCResponse
		if err := json.Unmarshal([]byte(data), &rpcResp); err != nil {
			continue // skip non-JSON data lines
		}
		if rpcResp.Error != nil {
			return nil, fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
		}
		if rpcResp.Result != nil {
			return rpcResp.Result, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read sse stream: %w", err)
	}
	return nil, fmt.Errorf("no result in sse stream")
}

func (h *HTTPInstance) resetIdleTimer() {
	if h.idleTimeout <= 0 {
		return
	}
	if h.idleTimer != nil {
		h.idleTimer.Stop()
	}
	h.idleTimer = time.AfterFunc(h.idleTimeout, func() {
		h.stop()
	})
}
