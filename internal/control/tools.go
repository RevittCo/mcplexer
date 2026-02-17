package control

import (
	"encoding/json"

	"github.com/revittco/mcplexer/internal/gateway"
)

func allTools() []gateway.Tool {
	return []gateway.Tool{
		// Server tools
		{
			Name:        "list_servers",
			Description: "List all downstream MCP servers",
			InputSchema: schema(nil, nil),
		},
		{
			Name:        "get_server",
			Description: "Get a downstream server by ID",
			InputSchema: schema(props{"id": propStr("Server ID")}, []string{"id"}),
		},
		{
			Name:        "create_server",
			Description: "Create a new downstream MCP server",
			InputSchema: schema(props{
				"name":             propStr("Unique server name"),
				"transport":        propStr("Transport type: stdio"),
				"command":          propStr("Command to run"),
				"args":             propArr("Command arguments"),
				"tool_namespace":   propStr("Tool namespace prefix"),
				"discovery":        propStr("Discovery mode: static or dynamic"),
				"idle_timeout_sec": propInt("Idle timeout in seconds"),
				"max_instances":    propInt("Maximum concurrent instances"),
				"restart_policy":   propStr("Restart policy: never, on-failure, always"),
			}, []string{"name", "command", "tool_namespace"}),
		},
		{
			Name:        "update_server",
			Description: "Update a downstream MCP server (partial update, only provided fields change)",
			InputSchema: schema(props{
				"id":               propStr("Server ID"),
				"name":             propStr("Unique server name"),
				"transport":        propStr("Transport type"),
				"command":          propStr("Command to run"),
				"args":             propArr("Command arguments"),
				"tool_namespace":   propStr("Tool namespace prefix"),
				"discovery":        propStr("Discovery mode"),
				"idle_timeout_sec": propInt("Idle timeout in seconds"),
				"max_instances":    propInt("Maximum concurrent instances"),
				"restart_policy":   propStr("Restart policy"),
			}, []string{"id"}),
		},
		{
			Name:        "delete_server",
			Description: "Delete a downstream MCP server",
			InputSchema: schema(props{"id": propStr("Server ID")}, []string{"id"}),
		},

		// Workspace tools
		{
			Name:        "list_workspaces",
			Description: "List all workspaces",
			InputSchema: schema(nil, nil),
		},
		{
			Name:        "get_workspace",
			Description: "Get a workspace by ID",
			InputSchema: schema(props{"id": propStr("Workspace ID")}, []string{"id"}),
		},
		{
			Name:        "create_workspace",
			Description: "Create a new workspace",
			InputSchema: schema(props{
				"name":           propStr("Unique workspace name"),
				"root_path":      propStr("Root file path for the workspace"),
				"default_policy": propStr("Default routing policy: allow or deny"),
				"tags":           propArr("Workspace tags"),
			}, []string{"name"}),
		},
		{
			Name:        "update_workspace",
			Description: "Update a workspace (partial update, only provided fields change)",
			InputSchema: schema(props{
				"id":             propStr("Workspace ID"),
				"name":           propStr("Unique workspace name"),
				"root_path":      propStr("Root file path"),
				"default_policy": propStr("Default routing policy"),
				"tags":           propArr("Workspace tags"),
			}, []string{"id"}),
		},
		{
			Name:        "delete_workspace",
			Description: "Delete a workspace",
			InputSchema: schema(props{"id": propStr("Workspace ID")}, []string{"id"}),
		},

		// Route rule tools
		{
			Name:        "list_routes",
			Description: "List route rules for a workspace",
			InputSchema: schema(props{
				"workspace_id": propStr("Workspace ID"),
			}, []string{"workspace_id"}),
		},
		{
			Name:        "create_route",
			Description: "Create a new route rule",
			InputSchema: schema(props{
				"priority":              propInt("Route priority (lower number = higher priority)"),
				"workspace_id":          propStr("Workspace ID"),
				"path_glob":             propStr("Path glob pattern"),
				"tool_match":            propObj("Tool match criteria"),
				"downstream_server_id":  propStr("Downstream server ID"),
				"auth_scope_id":         propStr("Auth scope ID"),
				"policy":                propStr("Policy: allow or deny"),
				"log_level":             propStr("Log level override"),
			}, []string{"workspace_id", "downstream_server_id", "policy"}),
		},
		{
			Name:        "update_route",
			Description: "Update a route rule (partial update, only provided fields change)",
			InputSchema: schema(props{
				"id":                    propStr("Route rule ID"),
				"priority":              propInt("Route priority"),
				"path_glob":             propStr("Path glob pattern"),
				"tool_match":            propObj("Tool match criteria"),
				"downstream_server_id":  propStr("Downstream server ID"),
				"auth_scope_id":         propStr("Auth scope ID"),
				"policy":                propStr("Policy"),
				"log_level":             propStr("Log level override"),
			}, []string{"id"}),
		},
		{
			Name:        "delete_route",
			Description: "Delete a route rule",
			InputSchema: schema(props{"id": propStr("Route rule ID")}, []string{"id"}),
		},

		// Auth scope tools
		{
			Name:        "list_auth_scopes",
			Description: "List all auth scopes",
			InputSchema: schema(nil, nil),
		},
		{
			Name:        "create_auth_scope",
			Description: "Create a new auth scope for downstream authentication",
			InputSchema: schema(props{
				"name": propStr("Unique scope name"),
				"type": propStr("Scope type (e.g. env, header)"),
			}, []string{"name", "type"}),
		},
		{
			Name:        "delete_auth_scope",
			Description: "Delete an auth scope",
			InputSchema: schema(props{"id": propStr("Auth scope ID")}, []string{"id"}),
		},

		// Info tools
		{
			Name:        "status",
			Description: "Get MCPlexer status with counts of servers, workspaces, sessions, and auth scopes",
			InputSchema: schema(nil, nil),
		},
		{
			Name:        "query_audit",
			Description: "Query audit log records with optional filters",
			InputSchema: schema(props{
				"tool_name": propStr("Filter by tool name"),
				"status":    propStr("Filter by status (success, error)"),
				"limit":     propInt("Max records to return (default 50)"),
				"offset":    propInt("Offset for pagination"),
			}, nil),
		},
	}
}

// adminTools is the set of tool names that require admin (read-write) access.
// These are blocked when the control server runs in read-only mode.
var adminTools = map[string]bool{
	"create_server":    true,
	"update_server":    true,
	"delete_server":    true,
	"create_workspace": true,
	"update_workspace": true,
	"delete_workspace": true,
	"create_route":     true,
	"update_route":     true,
	"delete_route":     true,
	"create_auth_scope": true,
	"delete_auth_scope": true,
}

// isAdminTool returns true if the tool requires admin access.
func isAdminTool(name string) bool {
	return adminTools[name]
}

// Schema helpers for building JSON Schema objects.

type props = map[string]any

func schema(properties map[string]any, required []string) json.RawMessage {
	s := map[string]any{"type": "object"}
	if properties != nil {
		s["properties"] = properties
	}
	if len(required) > 0 {
		s["required"] = required
	}
	data, _ := json.Marshal(s)
	return data
}

func propStr(desc string) map[string]string {
	return map[string]string{"type": "string", "description": desc}
}

func propInt(desc string) map[string]string {
	return map[string]string{"type": "integer", "description": desc}
}

func propArr(desc string) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": desc,
		"items":       map[string]string{"type": "string"},
	}
}

func propObj(desc string) map[string]string {
	return map[string]string{"type": "object", "description": desc}
}
