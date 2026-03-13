package codemode

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
)

// ToolCaller abstracts tool invocation so the sandbox can call through
// the full gateway pipeline (routing → auth → approval → cache → dispatch).
type ToolCaller interface {
	CallTool(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error)
}

// ToolCallRecord captures a single tool invocation for audit trail.
type ToolCallRecord struct {
	Name     string          `json:"name"`
	Args     json.RawMessage `json:"args"`
	Result   json.RawMessage `json:"result,omitempty"`
	Error    string          `json:"error,omitempty"`
	Duration time.Duration   `json:"duration_ms"`
}

// ExecutionResult contains everything produced by a sandbox execution.
type ExecutionResult struct {
	Output    string           `json:"output"`
	ToolCalls []ToolCallRecord `json:"tool_calls"`
	Error     string           `json:"error,omitempty"`
}

// Sandbox executes JavaScript code with tool functions bound as
// synchronous Go function calls. Each tool call routes through the
// full MCPlexer gateway pipeline.
type Sandbox struct {
	caller  ToolCaller
	timeout time.Duration
}

// NewSandbox creates a sandbox with the given tool caller and timeout.
func NewSandbox(caller ToolCaller, timeout time.Duration) *Sandbox {
	return &Sandbox{
		caller:  caller,
		timeout: timeout,
	}
}

// Execute runs JavaScript code in a fresh Goja VM with tool namespaces
// registered as global objects. Returns captured output and tool call records.
func (s *Sandbox) Execute(ctx context.Context, code string, tools []ToolDef) (*ExecutionResult, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	vm := goja.New()

	var (
		mu      sync.Mutex
		output  strings.Builder
		records []ToolCallRecord
	)

	// Register print() for captured output.
	if err := vm.Set("print", func(call goja.FunctionCall) goja.Value {
		parts := make([]string, len(call.Arguments))
		for i, arg := range call.Arguments {
			parts[i] = arg.String()
		}
		mu.Lock()
		output.WriteString(strings.Join(parts, " "))
		output.WriteByte('\n')
		mu.Unlock()
		return goja.Undefined()
	}); err != nil {
		return nil, fmt.Errorf("set print: %w", err)
	}

	// Register console.log as alias for print.
	console := vm.NewObject()
	if err := console.Set("log", func(call goja.FunctionCall) goja.Value {
		parts := make([]string, len(call.Arguments))
		for i, arg := range call.Arguments {
			parts[i] = arg.String()
		}
		mu.Lock()
		output.WriteString(strings.Join(parts, " "))
		output.WriteByte('\n')
		mu.Unlock()
		return goja.Undefined()
	}); err != nil {
		return nil, fmt.Errorf("set console.log: %w", err)
	}
	if err := vm.Set("console", console); err != nil {
		return nil, fmt.Errorf("set console: %w", err)
	}

	// Group tools by namespace and register each namespace as a global object.
	groups := groupByNamespace(tools)
	for ns, entries := range groups {
		nsObj := vm.NewObject()
		for _, entry := range entries {
			fullName := ns + "__" + entry.name
			fn := s.makeToolFunc(ctx, vm, fullName, &mu, &records)
			if err := nsObj.Set(entry.name, fn); err != nil {
				return nil, fmt.Errorf("set %s.%s: %w", ns, entry.name, err)
			}
		}
		if err := vm.Set(ns, nsObj); err != nil {
			return nil, fmt.Errorf("set namespace %s: %w", ns, err)
		}
	}

	// Run context cancellation watchdog — interrupts the VM on timeout.
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			vm.Interrupt("execution timeout")
		case <-done:
		}
	}()

	// Execute the code.
	_, err := vm.RunString(code)
	close(done)

	result := &ExecutionResult{
		Output:    output.String(),
		ToolCalls: records,
	}

	if err != nil {
		if isTimeoutError(err) {
			result.Error = "execution timed out"
			return result, nil
		}
		result.Error = err.Error()
		return result, nil
	}

	return result, nil
}

// makeToolFunc creates a Go function that Goja calls synchronously.
// The function marshals arguments, calls through the gateway pipeline,
// and returns the parsed result to JavaScript.
func (s *Sandbox) makeToolFunc(
	ctx context.Context,
	vm *goja.Runtime,
	toolName string,
	mu *sync.Mutex,
	records *[]ToolCallRecord,
) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		start := time.Now()

		// Marshal arguments from JS object to JSON.
		var argsJSON json.RawMessage
		if len(call.Arguments) > 0 {
			arg := call.Arguments[0]
			exported := arg.Export()
			data, err := json.Marshal(exported)
			if err != nil {
				panic(vm.ToValue(fmt.Sprintf("failed to marshal args for %s: %v", toolName, err)))
			}
			argsJSON = data
		} else {
			argsJSON = json.RawMessage("{}")
		}

		// Call through the full gateway pipeline (synchronous — blocks Goja).
		result, err := s.caller.CallTool(ctx, toolName, argsJSON)
		duration := time.Since(start)

		record := ToolCallRecord{
			Name:     toolName,
			Args:     argsJSON,
			Duration: duration,
		}

		if err != nil {
			record.Error = err.Error()
			mu.Lock()
			*records = append(*records, record)
			mu.Unlock()
			panic(vm.ToValue(fmt.Sprintf("tool call %s failed: %v", toolName, err)))
		}

		record.Result = result
		mu.Lock()
		*records = append(*records, record)
		mu.Unlock()

		// Parse the MCP CallToolResult to extract text content.
		return parseToolResult(vm, result)
	}
}

// parseToolResult extracts text content from an MCP CallToolResult and
// returns it as a Goja value. If the result is a JSON object with content
// array, it extracts and tries to parse the text as JSON.
func parseToolResult(vm *goja.Runtime, raw json.RawMessage) goja.Value {
	var callResult struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}

	if err := json.Unmarshal(raw, &callResult); err == nil && len(callResult.Content) > 0 {
		if callResult.IsError {
			panic(vm.ToValue(callResult.Content[0].Text))
		}

		text := callResult.Content[0].Text

		// Try to parse as JSON for structured data access.
		var parsed any
		if err := json.Unmarshal([]byte(text), &parsed); err == nil {
			return vm.ToValue(parsed)
		}

		return vm.ToValue(text)
	}

	// Fallback: try to parse the raw result as JSON.
	var parsed any
	if err := json.Unmarshal(raw, &parsed); err == nil {
		return vm.ToValue(parsed)
	}

	return vm.ToValue(string(raw))
}

// isTimeoutError checks if a Goja error was caused by an interrupt.
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "execution timeout") ||
		strings.Contains(msg, "context deadline exceeded")
}
