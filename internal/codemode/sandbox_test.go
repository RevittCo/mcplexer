package codemode

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// mockToolCaller records calls and returns canned responses.
type mockToolCaller struct {
	calls     []mockCall
	responses map[string]json.RawMessage
	errors    map[string]error
}

type mockCall struct {
	Name string
	Args json.RawMessage
}

func newMockCaller() *mockToolCaller {
	return &mockToolCaller{
		responses: make(map[string]json.RawMessage),
		errors:    make(map[string]error),
	}
}

func (m *mockToolCaller) CallTool(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	m.calls = append(m.calls, mockCall{Name: name, Args: args})
	if err, ok := m.errors[name]; ok {
		return nil, err
	}
	if resp, ok := m.responses[name]; ok {
		return resp, nil
	}
	return json.RawMessage(`{"content":[{"type":"text","text":"ok"}]}`), nil
}

func TestSandbox_Print(t *testing.T) {
	caller := newMockCaller()
	sandbox := NewSandbox(caller, 5*time.Second)

	result, err := sandbox.Execute(context.Background(), `print("hello world");`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Output != "hello world\n" {
		t.Errorf("expected 'hello world\\n', got %q", result.Output)
	}
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
}

func TestSandbox_ConsoleLog(t *testing.T) {
	caller := newMockCaller()
	sandbox := NewSandbox(caller, 5*time.Second)

	result, err := sandbox.Execute(context.Background(), `console.log("test");`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Output != "test\n" {
		t.Errorf("expected 'test\\n', got %q", result.Output)
	}
}

func TestSandbox_ToolCall(t *testing.T) {
	caller := newMockCaller()
	caller.responses["github__list_issues"] = json.RawMessage(
		`{"content":[{"type":"text","text":"[{\"id\":1,\"title\":\"bug\"}]"}]}`,
	)

	tools := []ToolDef{{
		Name: "github__list_issues",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {"owner": {"type": "string"}},
			"required": ["owner"]
		}`),
	}}

	sandbox := NewSandbox(caller, 5*time.Second)
	result, err := sandbox.Execute(context.Background(),
		`const issues = github.list_issues({ owner: "org" }); print(issues.length);`,
		tools,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(caller.calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(caller.calls))
	}
	if caller.calls[0].Name != "github__list_issues" {
		t.Errorf("expected github__list_issues, got %s", caller.calls[0].Name)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call record, got %d", len(result.ToolCalls))
	}
	if result.Output != "1\n" {
		t.Errorf("expected '1\\n', got %q", result.Output)
	}
}

func TestSandbox_MultiNamespace(t *testing.T) {
	caller := newMockCaller()
	caller.responses["github__list_issues"] = json.RawMessage(
		`{"content":[{"type":"text","text":"[{\"title\":\"fix\"}]"}]}`,
	)
	caller.responses["linear__create_issue"] = json.RawMessage(
		`{"content":[{"type":"text","text":"{\"id\":\"LIN-1\"}"}]}`,
	)

	tools := []ToolDef{
		{Name: "github__list_issues", InputSchema: json.RawMessage(`{"type":"object","properties":{}}`)},
		{Name: "linear__create_issue", InputSchema: json.RawMessage(`{"type":"object","properties":{}}`)},
	}

	sandbox := NewSandbox(caller, 5*time.Second)
	code := `
const issues = github.list_issues();
for (const issue of issues) {
  linear.create_issue({ title: issue.title });
}
print("synced " + issues.length);
`
	result, err := sandbox.Execute(context.Background(), code, tools)
	if err != nil {
		t.Fatal(err)
	}

	if len(caller.calls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(caller.calls))
	}
	if result.Output != "synced 1\n" {
		t.Errorf("expected 'synced 1\\n', got %q", result.Output)
	}
}

func TestSandbox_Timeout(t *testing.T) {
	caller := newMockCaller()
	sandbox := NewSandbox(caller, 100*time.Millisecond)

	result, err := sandbox.Execute(context.Background(), `while(true) {}`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Error != "execution timed out" {
		t.Errorf("expected timeout error, got %q", result.Error)
	}
}

func TestSandbox_ToolError(t *testing.T) {
	caller := newMockCaller()
	caller.errors["github__delete_repo"] = fmt.Errorf("permission denied")

	tools := []ToolDef{
		{Name: "github__delete_repo", InputSchema: json.RawMessage(`{"type":"object","properties":{}}`)},
	}

	sandbox := NewSandbox(caller, 5*time.Second)
	result, err := sandbox.Execute(context.Background(),
		`github.delete_repo();`,
		tools,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Error == "" {
		t.Error("expected error from failed tool call")
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call record, got %d", len(result.ToolCalls))
	}
	if result.ToolCalls[0].Error == "" {
		t.Error("expected error in tool call record")
	}
}

func TestSandbox_DataFiltering(t *testing.T) {
	caller := newMockCaller()
	// Simulate a large result that gets filtered in code.
	activities := make([]map[string]any, 100)
	for i := range activities {
		activities[i] = map[string]any{
			"id":   i,
			"type": "run",
		}
		if i%10 == 0 {
			activities[i]["type"] = "ride"
		}
	}
	data, _ := json.Marshal(activities)
	caller.responses["intervals__list_activities"] = json.RawMessage(
		fmt.Sprintf(`{"content":[{"type":"text","text":%s}]}`, string(mustMarshal(string(data)))),
	)

	tools := []ToolDef{
		{Name: "intervals__list_activities", InputSchema: json.RawMessage(`{"type":"object","properties":{}}`)},
	}

	sandbox := NewSandbox(caller, 5*time.Second)
	code := `
const all = intervals.list_activities();
const rides = all.filter(a => a.type === "ride");
print("rides: " + rides.length);
`
	result, err := sandbox.Execute(context.Background(), code, tools)
	if err != nil {
		t.Fatal(err)
	}
	if result.Output != "rides: 10\n" {
		t.Errorf("expected 'rides: 10\\n', got %q", result.Output)
	}
}

func TestSandbox_SyntaxError(t *testing.T) {
	caller := newMockCaller()
	sandbox := NewSandbox(caller, 5*time.Second)

	result, err := sandbox.Execute(context.Background(), `const x = ;`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Error == "" {
		t.Error("expected syntax error")
	}
}

func mustMarshal(v any) []byte {
	data, _ := json.Marshal(v)
	return data
}
