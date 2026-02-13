package gateway

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/revitteth/mcplexer/internal/approval"
	"github.com/revitteth/mcplexer/internal/audit"
	"github.com/revitteth/mcplexer/internal/downstream"
	"github.com/revitteth/mcplexer/internal/routing"
	"github.com/revitteth/mcplexer/internal/store"
)

// Server is the MCP gateway server.
type Server struct {
	handler *handler
	mu      sync.Mutex // protects stdout writes
}

// NewServer creates a new MCP gateway server.
func NewServer(
	s store.Store,
	engine *routing.Engine,
	manager *downstream.Manager,
	auditor *audit.Logger,
	transport TransportMode,
	opts ...ServerOption,
) *Server {
	var approvals *approval.Manager
	for _, o := range opts {
		o.apply(&approvals)
	}
	return &Server{
		handler: newHandler(s, engine, manager, auditor, transport, approvals),
	}
}

// ServerOption configures optional server features.
type ServerOption interface {
	apply(approvals **approval.Manager)
}

type withApprovals struct{ m *approval.Manager }

func (o withApprovals) apply(approvals **approval.Manager) { *approvals = o.m }

// WithApprovals enables the tool call approval system.
func WithApprovals(m *approval.Manager) ServerOption { return withApprovals{m} }

// RunStdio runs the MCP server over stdio (stdin/stdout).
func (s *Server) RunStdio(ctx context.Context) error {
	return s.run(ctx, os.Stdin, os.Stdout)
}

// RunConn runs the MCP server over an arbitrary reader/writer pair.
func (s *Server) RunConn(ctx context.Context, r io.Reader, w io.Writer) error {
	return s.run(ctx, r, w)
}

func (s *Server) run(ctx context.Context, r io.Reader, w io.Writer) error {
	defer s.handler.sessions.disconnect(ctx) //nolint:errcheck

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		resp := s.dispatch(ctx, line)
		if resp == nil {
			continue // notification, no response needed
		}

		if err := s.writeResponse(w, resp); err != nil {
			return fmt.Errorf("write response: %w", err)
		}
	}
	return scanner.Err()
}

func (s *Server) dispatch(ctx context.Context, line []byte) *Response {
	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		return &Response{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    CodeParseError,
				Message: "invalid JSON: " + err.Error(),
			},
		}
	}

	// Notifications have no ID; don't send a response.
	if req.ID == nil {
		s.handleNotification(req)
		return nil
	}

	var result json.RawMessage
	var rpcErr *RPCError

	switch req.Method {
	case "initialize":
		result, rpcErr = s.handler.handleInitialize(ctx, req.Params)
	case "ping":
		result, _ = json.Marshal(map[string]any{})
	case "tools/list":
		result, rpcErr = s.handler.handleToolsList(ctx)
	case "tools/call":
		result, rpcErr = s.handler.handleToolsCall(ctx, req.Params)
	default:
		rpcErr = &RPCError{
			Code:    CodeMethodNotFound,
			Message: fmt.Sprintf("unknown method: %s", req.Method),
		}
	}

	resp := &Response{JSONRPC: "2.0", ID: req.ID}
	if rpcErr != nil {
		resp.Error = rpcErr
	} else {
		resp.Result = result
	}
	return resp
}

func (s *Server) handleNotification(req Request) {
	switch req.Method {
	case "notifications/initialized":
		slog.Info("client initialized")
	default:
		slog.Debug("unhandled notification", "method", req.Method)
	}
}

func (s *Server) writeResponse(w io.Writer, resp *Response) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}
