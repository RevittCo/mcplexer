package downstream

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/revitteth/mcplexer/internal/auth"
	"github.com/revitteth/mcplexer/internal/store"
	"golang.org/x/sync/errgroup"
)

// downstream is the common interface for stdio and HTTP MCP instances.
type downstream interface {
	start(ctx context.Context) error
	stop()
	ListTools(ctx context.Context) (json.RawMessage, error)
	Call(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error)
	getState() InstanceState
}

// Manager orchestrates downstream MCP server process lifecycles.
type Manager struct {
	store     store.Store
	auth      *auth.Injector
	mu        sync.Mutex
	instances map[InstanceKey]downstream
}

// NewManager creates a new downstream process manager.
func NewManager(s store.Store, authInj *auth.Injector) *Manager {
	return &Manager{
		store:     s,
		auth:      authInj,
		instances: make(map[InstanceKey]downstream),
	}
}

// Call dispatches a tool call to the appropriate downstream instance.
// It lazy-starts the process if not already running.
func (m *Manager) Call(
	ctx context.Context,
	serverID, authScopeID, toolName string,
	args json.RawMessage,
) (json.RawMessage, error) {
	key := InstanceKey{ServerID: serverID, AuthScopeID: authScopeID}

	inst, err := m.getOrStart(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("get or start instance: %w", err)
	}

	params := mustMarshal(map[string]any{
		"name":      toolName,
		"arguments": json.RawMessage(args),
	})

	return inst.Call(ctx, "tools/call", params)
}

func (m *Manager) getOrStart(ctx context.Context, key InstanceKey) (downstream, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if inst, ok := m.instances[key]; ok {
		if inst.getState() != StateStopped {
			return inst, nil
		}
		// Instance stopped (idle timeout or crash); remove and restart.
		delete(m.instances, key)
	}

	inst, err := m.createInstance(ctx, key)
	if err != nil {
		return nil, err
	}

	if err := inst.start(ctx); err != nil {
		return nil, fmt.Errorf("start instance: %w", err)
	}

	m.instances[key] = inst
	return inst, nil
}

func (m *Manager) createInstance(
	ctx context.Context, key InstanceKey,
) (downstream, error) {
	server, err := m.store.GetDownstreamServer(ctx, key.ServerID)
	if err != nil {
		return nil, fmt.Errorf("get server %s: %w", key.ServerID, err)
	}

	if server.Disabled {
		return nil, fmt.Errorf("downstream server %q is disabled", server.Name)
	}

	timeout := time.Duration(server.IdleTimeoutSec) * time.Second

	if server.Transport == "http" && server.URL != nil {
		var headers http.Header
		if m.auth != nil && key.AuthScopeID != "" {
			var err error
			headers, err = m.auth.HeadersForDownstream(ctx, key.AuthScopeID)
			if err != nil {
				return nil, fmt.Errorf("resolve auth for scope %s: %w", key.AuthScopeID, err)
			}
		}
		return newHTTPInstance(key, *server.URL, timeout, headers), nil
	}

	// Default: stdio transport
	var cmdArgs []string
	if len(server.Args) > 0 {
		if err := json.Unmarshal(server.Args, &cmdArgs); err != nil {
			return nil, fmt.Errorf("unmarshal args: %w", err)
		}
	}

	var authEnv map[string]string
	if m.auth != nil {
		var err error
		authEnv, err = m.auth.EnvForDownstream(ctx, key.AuthScopeID)
		if err != nil {
			slog.Warn("failed to resolve auth env",
				"scope", key.AuthScopeID, "error", err)
		}
	}
	env := MergeEnv(os.Environ(), nil, authEnv)

	return newInstance(key, server.Command, cmdArgs, env, timeout), nil
}

// ListTools sends a tools/list request to a specific downstream instance.
func (m *Manager) ListTools(
	ctx context.Context, serverID, authScopeID string,
) (json.RawMessage, error) {
	key := InstanceKey{ServerID: serverID, AuthScopeID: authScopeID}

	inst, err := m.getOrStart(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("get or start instance: %w", err)
	}

	return inst.ListTools(ctx)
}

// ListAllTools queries all downstream servers for their tools in parallel.
// Returns a map of serverID -> raw tools/list result JSON.
func (m *Manager) ListAllTools(ctx context.Context) (map[string]json.RawMessage, error) {
	servers, err := m.store.ListDownstreamServers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list downstream servers: %w", err)
	}
	ids := make([]string, len(servers))
	for i, srv := range servers {
		ids[i] = srv.ID
	}
	return m.ListToolsForServers(ctx, ids)
}

// ListToolsForServers queries specific downstream servers for their tools in parallel.
func (m *Manager) ListToolsForServers(ctx context.Context, serverIDs []string) (map[string]json.RawMessage, error) {
	var mu sync.Mutex
	result := make(map[string]json.RawMessage, len(serverIDs))

	g, gCtx := errgroup.WithContext(ctx)
	for _, id := range serverIDs {
		g.Go(func() error {
			tools, err := m.ListTools(gCtx, id, "")
			if err != nil {
				slog.Warn("failed to list tools", "server", id, "error", err)
				return nil
			}
			mu.Lock()
			result[id] = tools
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return result, nil
}

// InstanceInfo describes a running downstream instance for status reporting.
type InstanceInfo struct {
	Key   InstanceKey
	State InstanceState
}

// ListInstances returns info about all tracked (non-stopped) instances.
func (m *Manager) ListInstances() []InstanceInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]InstanceInfo, 0, len(m.instances))
	for key, inst := range m.instances {
		s := inst.getState()
		if s == StateStopped {
			continue
		}
		out = append(out, InstanceInfo{Key: key, State: s})
	}
	return out
}

// Shutdown gracefully stops all running instances.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	instances := make([]downstream, 0, len(m.instances))
	for _, inst := range m.instances {
		instances = append(instances, inst)
	}
	m.mu.Unlock()

	for _, inst := range instances {
		inst.stop()
	}

	m.mu.Lock()
	m.instances = make(map[InstanceKey]downstream)
	m.mu.Unlock()
	return nil
}

// JSON-RPC types for downstream communication.

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func writeJSONLine(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
