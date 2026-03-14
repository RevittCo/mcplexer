package codemode

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGenerateTypeScript_SimpleTypes(t *testing.T) {
	tools := []ToolDef{
		{
			Name: "github__list_issues",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"owner": {"type": "string"},
					"repo": {"type": "string"},
					"state": {"type": "string", "enum": ["open", "closed", "all"]}
				},
				"required": ["owner", "repo"]
			}`),
		},
		{
			Name: "github__create_issue",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"owner": {"type": "string"},
					"repo": {"type": "string"},
					"title": {"type": "string"},
					"body": {"type": "string"}
				},
				"required": ["owner", "repo", "title"]
			}`),
		},
	}

	ts := GenerateTypeScript(tools)

	if !strings.Contains(ts, "declare namespace github") {
		t.Error("expected namespace github")
	}
	if !strings.Contains(ts, "interface ListIssuesParams") {
		t.Error("expected ListIssuesParams interface")
	}
	if !strings.Contains(ts, "owner: string") {
		t.Error("expected owner field")
	}
	if !strings.Contains(ts, `state?: "open" | "closed" | "all"`) {
		t.Error("expected state enum field")
	}
	if !strings.Contains(ts, "function list_issues(params: ListIssuesParams): any") {
		t.Error("expected list_issues function")
	}
	if !strings.Contains(ts, "declare function print(value: any): void") {
		t.Error("expected print declaration")
	}
}

func TestGenerateTypeScript_MultipleNamespaces(t *testing.T) {
	tools := []ToolDef{
		{
			Name: "github__list_repos",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {"org": {"type": "string"}},
				"required": ["org"]
			}`),
		},
		{
			Name: "linear__list_issues",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {"team_id": {"type": "string"}},
				"required": ["team_id"]
			}`),
		},
	}

	ts := GenerateTypeScript(tools)

	if !strings.Contains(ts, "declare namespace github") {
		t.Error("expected github namespace")
	}
	if !strings.Contains(ts, "declare namespace linear") {
		t.Error("expected linear namespace")
	}
}

func TestGenerateTypeScript_NoParams(t *testing.T) {
	tools := []ToolDef{
		{
			Name:        "github__whoami",
			InputSchema: json.RawMessage(`{"type": "object", "properties": {}}`),
		},
	}

	ts := GenerateTypeScript(tools)

	if !strings.Contains(ts, "function whoami(): any") {
		t.Error("expected no-params function")
	}
}

func TestGenerateTypeScript_ArrayType(t *testing.T) {
	tools := []ToolDef{
		{
			Name: "github__add_labels",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"labels": {"type": "array", "items": {"type": "string"}}
				},
				"required": ["labels"]
			}`),
		},
	}

	ts := GenerateTypeScript(tools)

	if !strings.Contains(ts, "labels: string[]") {
		t.Error("expected string[] type")
	}
}

func TestGenerateTypeScript_NestedObject(t *testing.T) {
	tools := []ToolDef{
		{
			Name: "api__create",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"config": {
						"type": "object",
						"properties": {
							"name": {"type": "string"},
							"count": {"type": "integer"}
						},
						"required": ["name"]
					}
				},
				"required": ["config"]
			}`),
		},
	}

	ts := GenerateTypeScript(tools)

	if !strings.Contains(ts, "config: { count?: number; name: string }") {
		t.Errorf("expected inline object type, got:\n%s", ts)
	}
}

func TestGenerateTypeScript_NumberTypes(t *testing.T) {
	tools := []ToolDef{
		{
			Name: "api__query",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"limit": {"type": "integer"},
					"score": {"type": "number"}
				}
			}`),
		},
	}

	ts := GenerateTypeScript(tools)

	if !strings.Contains(ts, "limit?: number") {
		t.Error("expected integer → number")
	}
	if !strings.Contains(ts, "score?: number") {
		t.Error("expected number type")
	}
}

func TestGenerateTypeScript_BooleanType(t *testing.T) {
	tools := []ToolDef{
		{
			Name: "api__toggle",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"enabled": {"type": "boolean"}
				}
			}`),
		},
	}

	ts := GenerateTypeScript(tools)

	if !strings.Contains(ts, "enabled?: boolean") {
		t.Error("expected boolean type")
	}
}

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"list_issues", "ListIssues"},
		{"create", "Create"},
		{"get_pr_comments", "GetPrComments"},
		{"whoami", "Whoami"},
	}

	for _, tt := range tests {
		got := toPascalCase(tt.input)
		if got != tt.want {
			t.Errorf("toPascalCase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestJsonSchemaToTS_AnyOf(t *testing.T) {
	raw := json.RawMessage(`{
		"anyOf": [
			{"type": "string"},
			{"type": "number"}
		]
	}`)

	got := jsonSchemaToTS(raw)
	if got != "string | number" {
		t.Errorf("expected 'string | number', got %q", got)
	}
}
