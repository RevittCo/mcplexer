package control

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/revitteth/mcplexer/internal/gateway"
	"github.com/revitteth/mcplexer/internal/store"
)

type handlerFunc func(ctx context.Context, s store.Store, args json.RawMessage) (json.RawMessage, error)

var handlers = map[string]handlerFunc{
	// Server
	"list_servers":  handleListServers,
	"get_server":    handleGetServer,
	"create_server": handleCreateServer,
	"update_server": handleUpdateServer,
	"delete_server": handleDeleteServer,
	// Workspace
	"list_workspaces":  handleListWorkspaces,
	"get_workspace":    handleGetWorkspace,
	"create_workspace": handleCreateWorkspace,
	"update_workspace": handleUpdateWorkspace,
	"delete_workspace": handleDeleteWorkspace,
	// Route
	"list_routes":  handleListRoutes,
	"create_route": handleCreateRoute,
	"update_route": handleUpdateRoute,
	"delete_route": handleDeleteRoute,
	// Auth
	"list_auth_scopes":  handleListAuthScopes,
	"create_auth_scope": handleCreateAuthScope,
	"delete_auth_scope": handleDeleteAuthScope,
	// Info
	"status":      handleStatus,
	"query_audit": handleQueryAudit,
}

// textResult wraps a text string in MCP CallToolResult format.
func textResult(text string) json.RawMessage {
	result := gateway.CallToolResult{
		Content: []gateway.ToolContent{{Type: "text", Text: text}},
	}
	data, _ := json.Marshal(result)
	return data
}

// jsonResult marshals v to indented JSON and wraps in MCP CallToolResult format.
func jsonResult(v any) (json.RawMessage, error) {
	text, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	return textResult(string(text)), nil
}

// errorResult wraps an error message in MCP CallToolResult format with isError=true.
func errorResult(msg string) json.RawMessage {
	result := gateway.CallToolResult{
		Content: []gateway.ToolContent{{Type: "text", Text: msg}},
		IsError: true,
	}
	data, _ := json.Marshal(result)
	return data
}

// requireID extracts and validates the "id" field from tool arguments.
func requireID(args json.RawMessage) (string, error) {
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}
	if p.ID == "" {
		return "", fmt.Errorf("id is required")
	}
	return p.ID, nil
}
