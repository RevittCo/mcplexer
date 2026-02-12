package gateway

import (
	"encoding/json"
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
		{Name: "ns__tool1", Description: "First tool", InputSchema: json.RawMessage(`{"type":"object"}`)},
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
	if !stringContains(result, `"type":"object"`) {
		t.Error("missing schema for tool1")
	}
}
