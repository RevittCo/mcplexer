package gateway

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestMatchesQuery(t *testing.T) {
	tests := []struct {
		name  string
		tool  Tool
		query string
		want  bool
	}{
		{
			name:  "name match",
			tool:  Tool{Name: "github__create_issue", Description: "Creates an issue"},
			query: "create",
			want:  true,
		},
		{
			name:  "description match",
			tool:  Tool{Name: "github__list_repos", Description: "List repositories"},
			query: "repositories",
			want:  true,
		},
		{
			name:  "no match",
			tool:  Tool{Name: "github__create_issue", Description: "Creates an issue"},
			query: "delete",
			want:  false,
		},
		{
			name:  "case insensitive name",
			tool:  Tool{Name: "GitHub__Create_Issue", Description: "Creates an issue"},
			query: "create_issue",
			want:  true,
		},
		{
			name:  "case insensitive description",
			tool:  Tool{Name: "github__list", Description: "List REPOSITORIES"},
			query: "repositories",
			want:  true,
		},
		{
			name:  "empty query matches all",
			tool:  Tool{Name: "anything", Description: "whatever"},
			query: "",
			want:  true,
		},
		// Multi-token fuzzy matching tests
		{
			name:  "multi-token matches across namespace and name",
			tool:  Tool{Name: "linear__list_tasks", Description: "List tasks from Linear"},
			query: "linear tasks",
			want:  true,
		},
		{
			name:  "multi-token matches namespace and description",
			tool:  Tool{Name: "linear__search_issues", Description: "Search issues and tasks in Linear"},
			query: "linear tasks",
			want:  true,
		},
		{
			name:  "multi-token all words must match",
			tool:  Tool{Name: "github__create_issue", Description: "Creates a GitHub issue"},
			query: "linear tasks",
			want:  false,
		},
		{
			name:  "multi-token matches with underscores expanded",
			tool:  Tool{Name: "slack__send_message", Description: "Send a message to a Slack channel"},
			query: "send message",
			want:  true,
		},
		{
			name:  "multi-token partial word match",
			tool:  Tool{Name: "jira__create_ticket", Description: "Create a Jira ticket"},
			query: "jira ticket",
			want:  true,
		},
		{
			name:  "multi-token with hyphens expanded",
			tool:  Tool{Name: "my-server__list-items", Description: "Lists items"},
			query: "list items",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesQuery(tt.tool, tt.query)
			if got != tt.want {
				t.Errorf("matchesQuery(%q, %q) = %v, want %v",
					tt.tool.Name, tt.query, got, tt.want)
			}
		})
	}
}

func TestScoreMatch(t *testing.T) {
	// Exact name match should score highest.
	exact := scoreMatch(Tool{Name: "linear__list_tasks", Description: "List tasks"}, "linear__list_tasks")
	partial := scoreMatch(Tool{Name: "linear__list_tasks", Description: "List tasks"}, "list")
	if exact <= partial {
		t.Errorf("exact name match (%d) should score higher than partial (%d)", exact, partial)
	}

	// Name match should score higher than description-only match.
	nameMatch := scoreMatch(Tool{Name: "github__create_issue", Description: "Creates an issue"}, "create_issue")
	descOnly := scoreMatch(Tool{Name: "github__list_repos", Description: "Create issue tracking"}, "create_issue")
	if nameMatch <= descOnly {
		t.Errorf("name match (%d) should score higher than desc-only (%d)", nameMatch, descOnly)
	}
}

func TestMarshalToolResult(t *testing.T) {
	result := marshalToolResult("hello world")
	var tr CallToolResult
	if err := json.Unmarshal(result, &tr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(tr.Content) != 1 {
		t.Fatalf("got %d content items, want 1", len(tr.Content))
	}
	if tr.Content[0].Type != "text" {
		t.Errorf("type = %q, want %q", tr.Content[0].Type, "text")
	}
	if tr.Content[0].Text != "hello world" {
		t.Errorf("text = %q, want %q", tr.Content[0].Text, "hello world")
	}
}

func TestFormatSearchResults(t *testing.T) {
	tools := []Tool{
		{
			Name:        "ns__tool1",
			Description: "First tool",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`),
		},
		{Name: "ns__tool2", Description: "Second tool"},
	}
	result := formatSearchResults(tools)
	if !stringContains(result, "Found 2 tools") {
		t.Errorf("missing count header in: %s", result)
	}
	if !stringContains(result, "ns__tool1") {
		t.Error("missing tool1 in result")
	}
	if !stringContains(result, "ns__tool2") {
		t.Error("missing tool2 in result")
	}
	// Should show compact param summary, not raw JSON schema
	if stringContains(result, `"type":"object"`) {
		t.Error("should not contain raw JSON schema")
	}
	if !stringContains(result, "query (required)") {
		t.Error("missing compact param summary for tool1")
	}
	if !stringContains(result, "load_tools") {
		t.Error("missing load_tools hint in search results")
	}
}

func TestSchemaParamSummary(t *testing.T) {
	tests := []struct {
		name   string
		schema json.RawMessage
		want   string
	}{
		{
			name:   "empty schema",
			schema: nil,
			want:   "",
		},
		{
			name:   "no properties",
			schema: json.RawMessage(`{"type":"object"}`),
			want:   "",
		},
		{
			name:   "required and optional",
			schema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"limit":{"type":"number"},"offset":{"type":"number"}},"required":["query"]}`),
			want:   "limit, offset, query (required)",
		},
		{
			name:   "all required",
			schema: json.RawMessage(`{"type":"object","properties":{"a":{"type":"string"},"b":{"type":"string"}},"required":["a","b"]}`),
			want:   "a (required), b (required)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := schemaParamSummary(tt.schema)
			if got != tt.want {
				t.Errorf("schemaParamSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatSearchResultsCapped(t *testing.T) {
	tools := make([]Tool, 25)
	for i := range tools {
		tools[i] = Tool{
			Name:        fmt.Sprintf("ns__tool_%02d", i),
			Description: fmt.Sprintf("Tool number %d", i),
		}
	}
	result := formatSearchResults(tools)
	if !stringContains(result, "Found 25 tools") {
		t.Errorf("missing total count in: %s", result)
	}
	if !stringContains(result, "showing first 20") {
		t.Error("missing cap message")
	}
	if !stringContains(result, "refine your query") {
		t.Error("missing refine message")
	}
	// tool_19 should be present (0-indexed, 20th item), tool_20 should not
	if !stringContains(result, "ns__tool_19") {
		t.Error("tool_19 should be included")
	}
	if stringContains(result, "ns__tool_20") {
		t.Error("tool_20 should be excluded (past cap)")
	}
}
