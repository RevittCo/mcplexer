package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/revittco/mcplexer/internal/routing"
	"github.com/revittco/mcplexer/internal/store"
)

const maxSearchResults = 20

// BuiltinPrefix is the namespace prefix for MCPlexer built-in tools.
const BuiltinPrefix = "mcpx__"

// legacyBuiltinPrefix is the old prefix, kept for backward compatibility.
const legacyBuiltinPrefix = "mcplexer__"

// isBuiltinTool returns true if the tool name uses the built-in prefix.
func isBuiltinTool(name string) bool {
	return strings.HasPrefix(name, BuiltinPrefix)
}

// normalizeBuiltinName converts legacy mcplexer__ prefixed names to mcpx__.
func normalizeBuiltinName(name string) string {
	if after, ok := strings.CutPrefix(name, legacyBuiltinPrefix); ok {
		return BuiltinPrefix + after
	}
	return name
}

// searchToolDefinition returns the built-in mcpx__search_tools Tool.
func searchToolDefinition() Tool {
	return Tool{
		Name:        "mcpx__search_tools",
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

	// Sort by relevance before filtering/capping.
	sort.Slice(matches, func(i, j int) bool {
		return scoreMatch(matches[i], queryLower) > scoreMatch(matches[j], queryLower)
	})

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
		return marshalToolResult(fmt.Sprintf("No tools found matching %q.", query)), nil
	}

	return marshalToolResult(formatSearchResults(matches)), nil
}

// filterByWorkspaceRoutes removes tools that the current session's workspace
// chain cannot route to. All tools (including built-ins) are subject to routing.
func (h *handler) filterByWorkspaceRoutes(ctx context.Context, tools []Tool) []Tool {
	ancestors := h.sessions.workspaceAncestors()

	filtered := make([]Tool, 0, len(tools))
	for _, t := range tools {
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

// buildSearchText creates a searchable string from a tool's name and description,
// expanding namespace separators, underscores, and hyphens into spaces so that
// token-based queries like "linear tasks" match tools named "linear__list_tasks".
func buildSearchText(t Tool) string {
	nameLower := strings.ToLower(t.Name)
	descLower := strings.ToLower(t.Description)

	expanded := strings.ReplaceAll(nameLower, "__", " ")
	expanded = strings.ReplaceAll(expanded, "_", " ")
	expanded = strings.ReplaceAll(expanded, "-", " ")

	return nameLower + " " + expanded + " " + descLower
}

// matchesQuery checks if a tool matches the search query. It first tries an
// exact substring match, then falls back to multi-token matching where every
// word in the query must appear somewhere in the tool's name or description.
func matchesQuery(t Tool, queryLower string) bool {
	if queryLower == "" {
		return true
	}

	searchText := buildSearchText(t)

	// Exact substring match (handles single-word and phrase queries).
	if strings.Contains(searchText, queryLower) {
		return true
	}

	// Multi-token: every query word must appear somewhere.
	tokens := strings.Fields(queryLower)
	if len(tokens) <= 1 {
		return false // single token already tried above
	}
	for _, tok := range tokens {
		if !strings.Contains(searchText, tok) {
			return false
		}
	}
	return true
}

// scoreMatch returns a relevance score for sorting search results (higher = better).
func scoreMatch(t Tool, queryLower string) int {
	nameLower := strings.ToLower(t.Name)
	descLower := strings.ToLower(t.Description)
	nameExpanded := strings.ReplaceAll(strings.ReplaceAll(nameLower, "__", " "), "_", " ")

	score := 0

	// Exact full match on name.
	if nameLower == queryLower || nameExpanded == queryLower {
		score += 100
	}
	// Name contains full query as substring.
	if strings.Contains(nameLower, queryLower) || strings.Contains(nameExpanded, queryLower) {
		score += 50
	}
	// Description contains full query as substring.
	if strings.Contains(descLower, queryLower) {
		score += 20
	}

	// Per-token scoring.
	for _, tok := range strings.Fields(queryLower) {
		if strings.Contains(nameLower, tok) || strings.Contains(nameExpanded, tok) {
			score += 10
		}
		if strings.Contains(descLower, tok) {
			score += 5
		}
	}
	return score
}

// formatSearchResults renders matched tools into compact human-readable text.
// Results are capped at maxSearchResults to avoid context bloat.
func formatSearchResults(tools []Tool) string {
	capped := len(tools) > maxSearchResults
	display := tools
	if capped {
		display = tools[:maxSearchResults]
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d tools", len(tools))
	if capped {
		fmt.Fprintf(&b, " (showing first %d — refine your query for more specific results)", maxSearchResults)
	}
	b.WriteString(":\n")

	for _, t := range display {
		params := schemaParamSummary(t.InputSchema)
		fmt.Fprintf(&b, "\n## %s\n%s\n", t.Name, t.Description)
		if params != "" {
			fmt.Fprintf(&b, "Parameters: %s\n", params)
		}
	}

	b.WriteString("\nUse load_tools with tool names or patterns to make them available.\n")
	return b.String()
}

// schemaParamSummary extracts a compact one-liner from a JSON schema, e.g.
// "query (required), limit, offset". Returns "" if the schema has no properties.
func schemaParamSummary(schema json.RawMessage) string {
	if len(schema) == 0 {
		return ""
	}
	var s struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(schema, &s); err != nil || len(s.Properties) == 0 {
		return ""
	}

	reqSet := make(map[string]bool, len(s.Required))
	for _, r := range s.Required {
		reqSet[r] = true
	}

	names := make([]string, 0, len(s.Properties))
	for name := range s.Properties {
		names = append(names, name)
	}
	sort.Strings(names)

	parts := make([]string, 0, len(names))
	for _, name := range names {
		if reqSet[name] {
			parts = append(parts, name+" (required)")
		} else {
			parts = append(parts, name)
		}
	}
	return strings.Join(parts, ", ")
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

// flushCacheToolDefinition returns the built-in MCP tool for flushing the cache.
func flushCacheToolDefinition() Tool {
	return Tool{
		Name:        "mcpx__flush_cache",
		Description: "Flush the tool call cache to force fresh data on subsequent calls. Use this when you suspect cached data is stale or after making changes that should be reflected immediately. Optionally specify a server_id to flush only that server's cache. Note: you can also pass `_cache_bust: true` as an argument to any individual tool call to bypass the cache for that specific request without flushing the entire cache.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"server_id": {
					"type": "string",
					"description": "Optional server ID to flush cache for a specific server only. Omit to flush all cached tool responses."
				}
			}
		}`),
	}
}

// approvalToolDefinitions returns the built-in MCP tools for the approval system.
func approvalToolDefinitions() []Tool {
	return []Tool{
		{
			Name:        "mcpx__list_pending_approvals",
			Description: "List pending tool call approvals waiting for review. Returns approval IDs, tool names, justifications, and requesting agent info. Your own pending requests are excluded.",
			InputSchema: json.RawMessage(`{"type": "object", "properties": {}}`),
		},
		{
			Name:        "mcpx__approve_tool_call",
			Description: "Approve a pending tool call request. You cannot approve your own requests.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"approval_id": {
						"type": "string",
						"description": "The ID of the pending approval to approve"
					},
					"reason": {
						"type": "string",
						"description": "Optional reason for approving"
					}
				},
				"required": ["approval_id"]
			}`),
		},
		{
			Name:        "mcpx__deny_tool_call",
			Description: "Deny a pending tool call request. You cannot deny your own requests. A reason is required.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"approval_id": {
						"type": "string",
						"description": "The ID of the pending approval to deny"
					},
					"reason": {
						"type": "string",
						"description": "Reason for denying the tool call"
					}
				},
				"required": ["approval_id", "reason"]
			}`),
		},
	}
}
