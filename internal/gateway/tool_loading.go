package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// loadToolDefinition returns the built-in mcpx__load_tools Tool.
func loadToolDefinition() Tool {
	return Tool{
		Name:        "mcpx__load_tools",
		Description: "Load tools into the active session by name or glob pattern (e.g. \"github__*\"). Loaded tools appear in tools/list. Use search_tools first to discover available tools.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"tools": {
					"type": "array",
					"items": {"type": "string"},
					"description": "Tool names or glob patterns to load"
				}
			},
			"required": ["tools"]
		}`),
	}
}

// unloadToolDefinition returns the built-in mcpx__unload_tools Tool.
func unloadToolDefinition() Tool {
	return Tool{
		Name:        "mcpx__unload_tools",
		Description: "Remove tools from the active session. Accepts tool names or glob patterns.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"tools": {
					"type": "array",
					"items": {"type": "string"},
					"description": "Tool names or glob patterns to unload"
				}
			},
			"required": ["tools"]
		}`),
	}
}

// handleLoadTools resolves tool patterns against all servers and adds matches
// to the session active set. Sends a tools/list_changed notification.
func (h *handler) handleLoadTools(ctx context.Context, patterns []string) (json.RawMessage, *RPCError) {
	if len(patterns) == 0 {
		return marshalErrorResult("No tool patterns provided."), nil
	}

	// Gather all available tools from all servers (live + cache).
	allTools, err := h.gatherAllTools(ctx)
	if err != nil {
		return nil, &RPCError{
			Code:    CodeInternalError,
			Message: fmt.Sprintf("gather tools: %v", err),
		}
	}

	// Match patterns against available tools.
	var matched []Tool
	for _, pattern := range patterns {
		for _, t := range allTools {
			if matchPattern(pattern, t.Name) {
				matched = append(matched, t)
			}
		}
	}

	// Filter by workspace routes.
	matched = h.filterByWorkspaceRoutes(ctx, matched)

	if len(matched) == 0 {
		return marshalToolResult("No matching tools found for the given patterns."), nil
	}

	// Add to session active set.
	h.sessions.loadTools(matched)

	// Notify client to re-fetch tools/list.
	h.sendToolsListChanged()

	// Build summary.
	names := make([]string, len(matched))
	for i, t := range matched {
		names[i] = t.Name
	}
	return marshalToolResult(fmt.Sprintf(
		"Loaded %d tool(s): %s", len(matched), strings.Join(names, ", "),
	)), nil
}

// handleUnloadTools removes tools from the session active set.
func (h *handler) handleUnloadTools(patterns []string) (json.RawMessage, *RPCError) {
	if len(patterns) == 0 {
		return marshalErrorResult("No tool patterns provided."), nil
	}

	// Resolve patterns against active tools.
	active := h.sessions.getActiveTools()
	var toRemove []string
	for _, pattern := range patterns {
		for _, t := range active {
			if matchPattern(pattern, t.Name) {
				toRemove = append(toRemove, t.Name)
			}
		}
	}

	if len(toRemove) == 0 {
		return marshalToolResult("No active tools matched the given patterns."), nil
	}

	removed := h.sessions.unloadTools(toRemove)
	h.sendToolsListChanged()

	return marshalToolResult(fmt.Sprintf("Unloaded %d tool(s).", removed)), nil
}

// gatherAllTools collects tools from all downstream servers (live + cache).
func (h *handler) gatherAllTools(ctx context.Context) ([]Tool, error) {
	servers, err := h.store.ListDownstreamServers(ctx)
	if err != nil {
		return nil, err
	}

	var serverIDs []string
	namespaces := make(map[string]string, len(servers))
	for _, srv := range servers {
		if srv.Transport == "internal" {
			continue // skip virtual servers
		}
		serverIDs = append(serverIDs, srv.ID)
		namespaces[srv.ID] = srv.ToolNamespace
	}

	liveTools, err := h.manager.ListToolsForServers(ctx, serverIDs)
	if err != nil {
		return nil, err
	}

	var all []Tool
	for _, srv := range servers {
		if srv.Transport == "internal" {
			continue
		}
		raw, ok := liveTools[srv.ID]
		if !ok {
			// Fall back to capabilities cache.
			if len(srv.CapabilitiesCache) > 0 && string(srv.CapabilitiesCache) != "{}" {
				raw = srv.CapabilitiesCache
			} else {
				continue
			}
		}
		ns := namespaces[srv.ID]
		tools, err := extractNamespacedTools(ns, raw)
		if err != nil {
			continue
		}
		all = append(all, tools...)
	}
	return all, nil
}

// sendToolsListChanged sends a tools/list_changed notification if a notifier is available.
func (h *handler) sendToolsListChanged() {
	if h.notifier == nil {
		return
	}
	if err := h.notifier.Notify("notifications/tools/list_changed", nil); err != nil {
		slog.Warn("failed to send tools/list_changed notification", "error", err)
	}
}

// matchPattern matches a tool name against a pattern that may contain * globs.
// Supports patterns like "github__*", "*__list_*", or exact names.
func matchPattern(pattern, name string) bool {
	if pattern == "*" {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return pattern == name
	}
	// Simple glob: split on * and match segments in order.
	parts := strings.Split(pattern, "*")
	remaining := name
	for i, part := range parts {
		if part == "" {
			continue
		}
		idx := strings.Index(remaining, part)
		if idx < 0 {
			return false
		}
		if i == 0 && idx != 0 {
			// First segment must match at start if pattern doesn't start with *.
			return false
		}
		remaining = remaining[idx+len(part):]
	}
	// If pattern doesn't end with *, remaining must be empty.
	if !strings.HasSuffix(pattern, "*") && remaining != "" {
		return false
	}
	return true
}
