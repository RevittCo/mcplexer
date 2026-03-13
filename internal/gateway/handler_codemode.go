package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/revittco/mcplexer/internal/codemode"
)

// handlerToolCaller adapts the gateway handler to the codemode.ToolCaller
// interface, routing each tool call through the full pipeline.
type handlerToolCaller struct {
	handler *handler
}

func (c *handlerToolCaller) CallTool(
	ctx context.Context, name string, args json.RawMessage,
) (json.RawMessage, error) {
	req := CallToolRequest{
		Name:      name,
		Arguments: args,
	}

	params, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal tool call: %w", err)
	}

	result, rpcErr := c.handler.handleToolsCall(ctx, params)
	if rpcErr != nil {
		return nil, fmt.Errorf("tool %s: %s", name, rpcErr.Message)
	}

	return result, nil
}

// handleCodeExecute runs user-provided code in a Goja sandbox with tool
// namespaces bound as synchronous function calls.
func (h *handler) handleCodeExecute(
	ctx context.Context, code string,
) (json.RawMessage, *RPCError) {
	timeout := h.codeModeTimeout(ctx)

	// Use cached tool defs to avoid re-querying all downstream servers.
	toolDefs, err := h.cachedCodeModeToolDefs(ctx)
	if err != nil {
		return nil, &RPCError{
			Code:    CodeInternalError,
			Message: fmt.Sprintf("gather tools for code mode: %v", err),
		}
	}

	// Strip TypeScript annotations to produce valid JS.
	jsCode := codemode.StripTypeScript(code)

	caller := &handlerToolCaller{handler: h}
	sandbox := codemode.NewSandbox(caller, timeout)

	result, err := sandbox.Execute(ctx, jsCode, toolDefs)
	if err != nil {
		return nil, &RPCError{
			Code:    CodeInternalError,
			Message: fmt.Sprintf("code execution failed: %v", err),
		}
	}

	slog.Info("code mode execution complete",
		"tool_calls", len(result.ToolCalls),
		"output_len", len(result.Output),
		"error", result.Error,
	)

	// Format the result as MCP tool output.
	return marshalCodeResult(result), nil
}

// handleGetCodeAPI returns TypeScript API definitions for loaded tools.
func (h *handler) handleGetCodeAPI(
	ctx context.Context, namespace string,
) (json.RawMessage, *RPCError) {
	tools, err := h.gatherCodeModeTools(ctx)
	if err != nil {
		return nil, &RPCError{
			Code:    CodeInternalError,
			Message: fmt.Sprintf("gather tools for code API: %v", err),
		}
	}

	// Filter by namespace if specified.
	if namespace != "" {
		var filtered []Tool
		for _, t := range tools {
			if ns, _, ok := splitNamespace(t.Name); ok && ns == namespace {
				filtered = append(filtered, t)
			}
		}
		tools = filtered
	}

	if len(tools) == 0 {
		if namespace != "" {
			return marshalToolResult(fmt.Sprintf(
				"No tools found for namespace %q. Use search_tools to discover available namespaces.", namespace,
			)), nil
		}
		return marshalToolResult("No tools available for code API."), nil
	}

	// Convert to ToolDefs and generate TypeScript.
	toolDefs := make([]codemode.ToolDef, len(tools))
	for i, t := range tools {
		toolDefs[i] = codemode.ToolDef{
			Name:        t.Name,
			InputSchema: t.InputSchema,
		}
	}

	ts := codemode.GenerateTypeScript(toolDefs)
	return marshalToolResult(ts), nil
}

// gatherCodeModeTools collects all tools available for code mode, including
// static tools, active session tools, and addon tools — but excluding
// built-in mcpx__ tools.
func (h *handler) gatherCodeModeTools(ctx context.Context) ([]Tool, error) {
	allTools, err := h.gatherAllTools(ctx)
	if err != nil {
		return nil, err
	}

	// Include active session tools.
	allTools = append(allTools, h.sessions.getActiveTools()...)

	// Deduplicate and exclude built-in tools.
	seen := make(map[string]struct{})
	var filtered []Tool
	for _, t := range allTools {
		if _, ok := seen[t.Name]; ok {
			continue
		}
		seen[t.Name] = struct{}{}
		// Exclude mcpx__ built-in tools from code API.
		if ns, _, ok := splitNamespace(t.Name); ok && ns == "mcpx" {
			continue
		}
		filtered = append(filtered, t)
	}

	// Filter by workspace routes.
	filtered = h.filterByWorkspaceRoutes(ctx, filtered)

	return filtered, nil
}

// cachedCodeModeToolDefs returns tool definitions for the sandbox, using
// the tools/list cache to avoid re-querying downstream servers on every
// execute_code call. Cache TTL matches the tools/list cache (default 15s).
func (h *handler) cachedCodeModeToolDefs(ctx context.Context) ([]codemode.ToolDef, error) {
	const cacheKey = "__codemode_tooldefs__"

	cached, err := h.toolsListCache.GetOrLoad(cacheKey, func() (json.RawMessage, error) {
		tools, err := h.gatherCodeModeTools(ctx)
		if err != nil {
			return nil, err
		}
		defs := toolsToToolDefs(tools)
		data, err := json.Marshal(defs)
		if err != nil {
			return nil, err
		}
		return data, nil
	})
	if err != nil {
		return nil, err
	}

	var defs []codemode.ToolDef
	if err := json.Unmarshal(cached, &defs); err != nil {
		return nil, err
	}
	return defs, nil
}

// toolsToToolDefs converts gateway Tools to codemode ToolDefs.
func toolsToToolDefs(tools []Tool) []codemode.ToolDef {
	defs := make([]codemode.ToolDef, len(tools))
	for i, t := range tools {
		defs[i] = codemode.ToolDef{
			Name:        t.Name,
			InputSchema: t.InputSchema,
		}
	}
	return defs
}

// splitNamespace splits "namespace__name" into its parts.
func splitNamespace(name string) (string, string, bool) {
	for i := 0; i < len(name)-1; i++ {
		if name[i] == '_' && name[i+1] == '_' {
			return name[:i], name[i+2:], true
		}
	}
	return "", name, false
}

// marshalCodeResult formats an ExecutionResult as MCP CallToolResult.
func marshalCodeResult(result *codemode.ExecutionResult) json.RawMessage {
	var content []ToolContent

	if result.Output != "" {
		content = append(content, ToolContent{
			Type: "text",
			Text: result.Output,
		})
	}

	if result.Error != "" {
		content = append(content, ToolContent{
			Type: "text",
			Text: fmt.Sprintf("Error: %s", result.Error),
		})
	}

	// Add summary of tool calls.
	if len(result.ToolCalls) > 0 {
		summary := formatToolCallSummary(result.ToolCalls)
		content = append(content, ToolContent{
			Type: "text",
			Text: summary,
		})
	}

	if len(content) == 0 {
		content = append(content, ToolContent{
			Type: "text",
			Text: "Code executed successfully with no output.",
		})
	}

	callResult := CallToolResult{
		Content: content,
		IsError: result.Error != "",
	}

	data, _ := json.Marshal(callResult)
	return data
}

// formatToolCallSummary creates a compact summary of tool calls.
func formatToolCallSummary(calls []codemode.ToolCallRecord) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n--- %d tool call(s) executed ---", len(calls))
	for i, call := range calls {
		status := "ok"
		if call.Error != "" {
			status = "error: " + call.Error
		}
		fmt.Fprintf(&b, "\n%d. %s (%s, %dms)",
			i+1, call.Name, status, call.Duration.Milliseconds())
	}
	return b.String()
}

// codeModeEnabled checks if code mode is enabled in settings.
func (h *handler) codeModeEnabled(ctx context.Context) bool {
	if h.settingsSvc == nil {
		return false
	}
	return h.settingsSvc.Load(ctx).CodeModeEnabled
}

// codeModeTimeout returns the configured timeout for code execution.
func (h *handler) codeModeTimeout(ctx context.Context) time.Duration {
	timeout := 30 // default
	if h.settingsSvc != nil {
		if t := h.settingsSvc.Load(ctx).CodeModeTimeoutSec; t > 0 {
			timeout = t
		}
	}
	return time.Duration(timeout) * time.Second
}

// maxEmbeddedAPISize is the max TypeScript API size to embed directly in the
// execute_code tool description. Beyond this, the AI should call get_code_api.
const maxEmbeddedAPISize = 8000

// buildCodeExecuteTool generates the execute_code tool definition with the
// TypeScript API embedded directly in the description when it fits. Returns
// the tool and whether the API was successfully embedded (true = no need for
// get_code_api as a separate tool).
func (h *handler) buildCodeExecuteTool(ctx context.Context) (Tool, bool) {
	var description string
	apiEmbedded := false

	toolDefs, err := h.cachedCodeModeToolDefs(ctx)
	if err != nil || len(toolDefs) == 0 {
		description = "Execute JavaScript/TypeScript code. " +
			"Call get_code_api first to see available tool functions and their signatures. " +
			"Tool functions are synchronous (no await). Use print() for output."
	} else {
		tsAPI := codemode.GenerateTypeScript(toolDefs)

		if len(tsAPI) <= maxEmbeddedAPISize {
			description = "Execute JavaScript code with the tool API below. " +
				"All functions are synchronous — no await needed. Use print() for output.\n\n" +
				tsAPI
			apiEmbedded = true
		} else {
			// Too large to embed — summarize namespaces and tell AI to use get_code_api.
			nsCounts := make(map[string]int)
			for _, td := range toolDefs {
				if ns, _, ok := splitNamespace(td.Name); ok {
					nsCounts[ns]++
				}
			}
			var sb strings.Builder
			for ns, count := range nsCounts {
				fmt.Fprintf(&sb, "  %s (%d tools)\n", ns, count)
			}
			description = fmt.Sprintf(
				"Execute JavaScript code with access to %d tools across these namespaces:\n%s"+
					"Call get_code_api to see TypeScript signatures (optionally filter by namespace). "+
					"All functions are synchronous — no await needed. Use print() for output.",
				len(toolDefs), sb.String(),
			)
		}
	}

	return Tool{
		Name:        "mcpx__execute_code",
		Description: description,
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"code": {
					"type": "string",
					"description": "JavaScript or TypeScript code to execute. TypeScript type annotations are automatically stripped. Tool functions are available as namespace.function_name() — call them synchronously without await."
				}
			},
			"required": ["code"]
		}`),
		Extras: withAnnotations(ToolAnnotations{
			Title:           "Execute Code",
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(false),
		}),
	}, apiEmbedded
}

// codeAPIToolDefinition returns the MCP tool definition for retrieving code API docs.
func codeAPIToolDefinition() Tool {
	return Tool{
		Name: "mcpx__get_code_api",
		Description: "Get TypeScript API definitions for all available tool functions. " +
			"Returns type declarations showing function signatures, parameter types, " +
			"and namespaces. Use this before execute_code to understand the available API. " +
			"Optionally filter by namespace (e.g. 'github', 'linear').",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"namespace": {
					"type": "string",
					"description": "Optional namespace to filter (e.g. 'github'). Omit for all namespaces."
				}
			}
		}`),
		Extras: withAnnotations(ToolAnnotations{
			Title:           "Get Code API",
			ReadOnlyHint:    boolPtr(true),
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(false),
		}),
	}
}
