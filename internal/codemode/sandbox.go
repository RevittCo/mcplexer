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

	// Shared print handler used by both print() and console.log.
	printFn := makePrintFunc(&mu, &output)

	if err := vm.Set("print", printFn); err != nil {
		return nil, fmt.Errorf("set print: %w", err)
	}

	console := vm.NewObject()
	if err := console.Set("log", printFn); err != nil {
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
		defer func() {
			if r := recover(); r != nil {
				switch v := r.(type) {
				case *goja.Exception:
					panic(v)
				case goja.Value:
					panic(v)
				case error:
					panic(vm.ToValue(fmt.Sprintf("tool call %s panicked: %v", toolName, v)))
				default:
					panic(vm.ToValue(fmt.Sprintf("tool call %s panicked: %v", toolName, v)))
				}
			}
		}()

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

		// Parse the MCP CallToolResult into the most useful JS value we can.
		return parseToolResult(vm, result)
	}
}

// parseToolResult converts an MCP CallToolResult to a Goja value.
// For a single text item it returns the parsed JSON payload when possible,
// otherwise the raw text. For richer content it returns the full call result.
func parseToolResult(vm *goja.Runtime, raw json.RawMessage) goja.Value {
	var callResult struct {
		Content []map[string]any `json:"content"`
		IsError bool             `json:"isError"`
	}

	if err := json.Unmarshal(raw, &callResult); err == nil {
		if callResult.IsError {
			panic(vm.ToValue(formatToolError(callResult.Content, raw)))
		}

		if len(callResult.Content) == 1 {
			if text, ok := callResult.Content[0]["text"].(string); ok {
				var parsed any
				if err := json.Unmarshal([]byte(text), &parsed); err == nil {
					return vm.ToValue(parsed)
				}
				return vm.ToValue(text)
			}
		}

		var parsed any
		if err := json.Unmarshal(raw, &parsed); err == nil {
			return vm.ToValue(parsed)
		}
	}

	var parsed any
	if err := json.Unmarshal(raw, &parsed); err == nil {
		return vm.ToValue(parsed)
	}

	return vm.ToValue(string(raw))
}

func formatToolError(content []map[string]any, raw json.RawMessage) string {
	parts := make([]string, 0, len(content))
	for _, item := range content {
		if text, ok := item["text"].(string); ok && text != "" {
			parts = append(parts, text)
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, "\n")
	}
	if len(raw) > 0 {
		return string(raw)
	}
	return "tool returned error"
}

// makePrintFunc returns a Goja-compatible function that captures output.
// Objects and arrays are auto-serialized to indented JSON so callers
// never see "[object Object]".
func makePrintFunc(mu *sync.Mutex, output *strings.Builder) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		parts := make([]string, len(call.Arguments))
		for i, arg := range call.Arguments {
			parts[i] = formatPrintArg(arg)
		}
		mu.Lock()
		output.WriteString(strings.Join(parts, " "))
		output.WriteByte('\n')
		mu.Unlock()
		return goja.Undefined()
	}
}

// formatPrintArg converts a Goja value to a readable string.
// Primitives use their natural string form; objects and arrays are
// JSON-serialized with indentation for readability.
func formatPrintArg(arg goja.Value) string {
	if arg == nil || goja.IsUndefined(arg) || goja.IsNull(arg) {
		return arg.String()
	}
	exported := arg.Export()
	switch exported.(type) {
	case string, float64, int64, bool:
		return arg.String()
	default:
		data, err := json.MarshalIndent(exported, "", "  ")
		if err != nil {
			return arg.String()
		}
		return string(data)
	}
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
