package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/revitteth/mcplexer/internal/routing"
	"github.com/revitteth/mcplexer/internal/store"
)

// searchToolDefinition returns the built-in mcplexer__search_tools Tool.
func searchToolDefinition() Tool {
	return Tool{
		Name:        "mcplexer__search_tools",
		Description: "Search for available tools across all connected MCP servers. Use this to discover tools that aren't listed by default. Returns tool names, descriptions, and input schemas.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {
					"type": "string",
					"description": "Search query to match against tool names and descriptions"
				}
			},
			"required": ["query"]
		}`),
	}
}

// handleSearchTools queries all dynamic-discovery servers and returns matching tools.
// Results are filtered by the current session's workspace routes when a workspace is bound.
func (h *handler) handleSearchTools(ctx context.Context, query string) (json.RawMessage, *RPCError) {
	servers, err := h.store.ListDownstreamServers(ctx)
	if err != nil {
		return nil, &RPCError{Code: CodeInternalError, Message: fmt.Sprintf("list servers: %v", err)}
	}

	var dynamicIDs []string
	dynamicServers := make(map[string]store.DownstreamServer)
	namespaces := make(map[string]string)
	for _, srv := range servers {
		if srv.Discovery == "dynamic" {
			dynamicIDs = append(dynamicIDs, srv.ID)
			dynamicServers[srv.ID] = srv
			namespaces[srv.ID] = srv.ToolNamespace
		}
	}

	if len(dynamicIDs) == 0 {
		return marshalErrorResult("No dynamic servers configured."), nil
	}

	liveTools, err := h.manager.ListToolsForServers(ctx, dynamicIDs)
	if err != nil {
		return nil, &RPCError{Code: CodeInternalError, Message: fmt.Sprintf("search tools: %v", err)}
	}

	queryLower := strings.ToLower(query)
	var matches []Tool

	// Try live results first, then fall back to capabilities cache for
	// servers that failed (e.g. HTTP servers requiring auth for listing).
	for _, id := range dynamicIDs {
		rawResult, ok := liveTools[id]
		if !ok {
			srv := dynamicServers[id]
			if len(srv.CapabilitiesCache) > 0 && string(srv.CapabilitiesCache) != "{}" {
				rawResult = srv.CapabilitiesCache
			} else {
				continue
			}
		} else {
			if err := h.store.UpdateCapabilitiesCache(ctx, id, rawResult); err != nil {
				slog.Warn("failed to update capabilities cache", "server", id, "error", err)
			}
		}

		ns := namespaces[id]
		tools, err := extractNamespacedTools(ns, rawResult)
		if err != nil {
			continue
		}
		for _, t := range tools {
			if matchesQuery(t, queryLower) {
				matches = append(matches, t)
			}
		}
	}

	// Filter results by workspace routes to prevent information leakage.
	preFilterCount := len(matches)
	matches = h.filterByWorkspaceRoutes(ctx, matches)
	blocked := preFilterCount - len(matches)

	if len(matches) == 0 {
		if blocked > 0 {
			return marshalErrorResult(fmt.Sprintf(
				"Blocked: %d tools matched %q but none are allowed by your workspace routes.",
				blocked, query)), nil
		}
		return marshalErrorResult(fmt.Sprintf("No tools found matching %q.", query)), nil
	}

	return marshalToolResult(formatSearchResults(matches)), nil
}

// filterByWorkspaceRoutes removes tools that the current session's workspace
// chain cannot route to. Built-in mcplexer tools are always included. If no
// workspace is bound, only built-in tools are returned.
func (h *handler) filterByWorkspaceRoutes(ctx context.Context, tools []Tool) []Tool {
	ancestors := h.sessions.workspaceAncestors()

	filtered := make([]Tool, 0, len(tools))
	for _, t := range tools {
		// Built-in tools are always visible.
		if strings.HasPrefix(t.Name, "mcplexer__") {
			filtered = append(filtered, t)
			continue
		}
		if len(ancestors) == 0 {
			continue
		}
		_, err := h.engine.RouteWithFallback(ctx, routing.RouteContext{
			ToolName: t.Name,
		}, h.sessions.clientRoot(), ancestors)
		if err == nil {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// matchesQuery checks if a tool's name or description contains the query.
func matchesQuery(t Tool, queryLower string) bool {
	return strings.Contains(strings.ToLower(t.Name), queryLower) ||
		strings.Contains(strings.ToLower(t.Description), queryLower)
}

// formatSearchResults renders matched tools into human-readable text.
func formatSearchResults(tools []Tool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Found %d tools:\n", len(tools))
	for _, t := range tools {
		fmt.Fprintf(&b, "\n## %s\n%s\n", t.Name, t.Description)
		if len(t.InputSchema) > 0 {
			fmt.Fprintf(&b, "Input schema: %s\n", string(t.InputSchema))
		}
	}
	return b.String()
}

// marshalToolResult wraps text into MCP CallToolResult format.
func marshalToolResult(text string) json.RawMessage {
	result := CallToolResult{
		Content: []ToolContent{{Type: "text", Text: text}},
	}
	data, _ := json.Marshal(result)
	return data
}

// marshalErrorResult wraps text into MCP CallToolResult with isError=true.
func marshalErrorResult(text string) json.RawMessage {
	result := CallToolResult{
		Content: []ToolContent{{Type: "text", Text: text}},
		IsError: true,
	}
	data, _ := json.Marshal(result)
	return data
}
