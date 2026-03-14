package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/revittco/mcplexer/internal/codemode"
	"github.com/revittco/mcplexer/internal/store"
)

type codeModeContextKey string

const internalCodeModeCallKey codeModeContextKey = "internal-code-mode-call"

func withInternalCodeModeCall(ctx context.Context) context.Context {
	return context.WithValue(ctx, internalCodeModeCallKey, true)
}

func isInternalCodeModeCall(ctx context.Context) bool {
	internal, _ := ctx.Value(internalCodeModeCallKey).(bool)
	return internal
}

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

	toolDefs, err := h.codeModeToolDefs(ctx)
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

	result, err := sandbox.Execute(withInternalCodeModeCall(ctx), jsCode, toolDefs)
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

// handleGetCodeAPI returns TypeScript API definitions for code-mode tools.
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
				"No tools found for namespace %q. Call get_code_api without a namespace filter to inspect the full API.", namespace,
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

// gatherCodeModeTools collects all tools available through execute_code.
func (h *handler) gatherCodeModeTools(ctx context.Context) ([]Tool, error) {
	servers, err := h.store.ListDownstreamServers(ctx)
	if err != nil {
		return nil, err
	}

	var (
		staticServers  []store.DownstreamServer
		dynamicServers []store.DownstreamServer
		namespaces     = make(map[string]string, len(servers))
		allTools       []Tool
	)

	for _, srv := range servers {
		if srv.Transport == "internal" {
			continue
		}
		namespaces[srv.ID] = srv.ToolNamespace
		if srv.Discovery == "dynamic" {
			dynamicServers = append(dynamicServers, srv)
		} else {
			staticServers = append(staticServers, srv)
		}
	}

	collect := func(serverGroup []store.DownstreamServer) error {
		if len(serverGroup) == 0 {
			return nil
		}

		serverIDs := make([]string, 0, len(serverGroup))
		for _, srv := range serverGroup {
			serverIDs = append(serverIDs, srv.ID)
		}

		liveTools, err := h.cachedListToolsForServers(ctx, serverIDs)
		if err != nil {
			return err
		}

		for _, srv := range serverGroup {
			rawResult, ok := liveTools[srv.ID]
			if !ok {
				if len(srv.CapabilitiesCache) > 0 && string(srv.CapabilitiesCache) != "{}" {
					rawResult = srv.CapabilitiesCache
				} else {
					continue
				}
			} else if err := h.store.UpdateCapabilitiesCache(ctx, srv.ID, rawResult); err != nil {
				slog.Warn("failed to update capabilities cache",
					"server", srv.ID, "error", err)
			}

			ns := namespaces[srv.ID]
			tools, err := extractNamespacedTools(ns, rawResult)
			if err != nil {
				slog.Warn("failed to extract code mode tools",
					"server", srv.ID, "error", err)
				continue
			}
			allTools = append(allTools, tools...)
		}

		return nil
	}

	if err := collect(staticServers); err != nil {
		return nil, err
	}
	if err := collect(dynamicServers); err != nil {
		return nil, err
	}

	if h.addonRegistry != nil {
		allTools = append(allTools, addonToolDefinitions(h.addonRegistry)...)
	}
	allTools = append(allTools, h.codeModeBuiltinTools(len(dynamicServers) > 0)...)

	seen := make(map[string]struct{})
	var filtered []Tool
	for _, t := range allTools {
		if _, ok := seen[t.Name]; ok {
			continue
		}
		seen[t.Name] = struct{}{}
		filtered = append(filtered, t)
	}

	filtered = h.filterByWorkspaceRoutes(ctx, filtered)

	return filtered, nil
}

func (h *handler) codeModeToolDefs(ctx context.Context) ([]codemode.ToolDef, error) {
	tools, err := h.gatherCodeModeTools(ctx)
	if err != nil {
		return nil, err
	}
	return toolsToToolDefs(tools), nil
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

func (h *handler) codeModeBuiltinTools(hasDynamicServers bool) []Tool {
	var tools []Tool
	if hasDynamicServers {
		tools = append(tools, searchToolDefinition())
	}
	if h.approvals != nil {
		tools = append(tools, approvalToolDefinitions()...)
	}
	if _, ok := h.manager.(CachingCaller); ok {
		tools = append(tools, flushCacheToolDefinition())
	}
	return tools
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

	toolDefs, err := h.codeModeToolDefs(ctx)
	if err != nil || len(toolDefs) == 0 {
		description = "Execute JavaScript code that calls multiple tools in a single invocation. " +
			"Combine ALL related queries into a single script — calls run sequentially, " +
			"so results from earlier calls feed directly into later ones (daisy-chain). " +
			"Call get_code_api first to inspect available functions. " +
			"All calls are synchronous (no await). " +
			"print() auto-serializes objects to JSON — no JSON.stringify needed."
	} else {
		tsAPI := codemode.GenerateTypeScript(toolDefs)

		if len(tsAPI) <= maxEmbeddedAPISize {
			description = "Execute JavaScript code that batches multiple tool calls into one invocation. " +
				"Combine ALL related queries into a single script — calls run sequentially, " +
				"so results from earlier calls feed directly into later ones (daisy-chain). " +
				"Avoid multiple execute_code calls when one script will do. " +
				"All functions are synchronous — no await needed. " +
				"print() auto-serializes objects to JSON — no JSON.stringify needed. " +
				"The declarations below are reference-only and should not be pasted into execute_code.\n\n" +
				tsAPI
			apiEmbedded = true
		} else {
			nsCounts := make(map[string]int)
			for _, td := range toolDefs {
				if ns, _, ok := splitNamespace(td.Name); ok {
					nsCounts[ns]++
				}
			}
			var sb strings.Builder
			namespaces := make([]string, 0, len(nsCounts))
			for ns := range nsCounts {
				namespaces = append(namespaces, ns)
			}
			sort.Strings(namespaces)
			for _, ns := range namespaces {
				count := nsCounts[ns]
				fmt.Fprintf(&sb, "  %s (%d tools)\n", ns, count)
			}
			description = fmt.Sprintf(
				"Execute JavaScript code that batches multiple tool calls into one invocation. "+
					"Combine ALL related queries into a single script — calls run sequentially, "+
					"so results from earlier calls feed directly into later ones (daisy-chain). "+
					"Avoid multiple execute_code calls when one script will do. "+
					"Access to %d tools across these namespaces:\n%s"+
					"Call get_code_api to see TypeScript signatures (optionally filter by namespace). "+
					"All functions are synchronous — no await needed. "+
					"print() auto-serializes objects to JSON — no JSON.stringify needed.",
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
						"description": "JavaScript code to execute. Combine ALL tool calls into one script — calls run sequentially and return values from earlier calls can be passed directly to later ones (daisy-chain). Tool functions are available as namespace.function_name() — call them synchronously without await. print() auto-serializes objects to JSON. Basic TypeScript annotations are stripped; the declarations from get_code_api are reference-only."
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
			"and namespaces. Review these before writing execute_code scripts that batch " +
			"multiple tool calls together. Optionally filter by namespace (e.g. 'github', 'linear').",
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
