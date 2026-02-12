package gateway

import "encoding/json"

// JSON-RPC 2.0 types.

// Request is a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError is a JSON-RPC 2.0 error.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// JSON-RPC error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603

	// MCP-specific error codes.
	CodeRouteNotFound = -32003
	CodeProcessError  = -32002
	CodeTimeout       = -32001
)

// MCP-specific types.

// InitializeParams is the client's initialize request params.
type InitializeParams struct {
	ProtocolVersion string     `json:"protocolVersion"`
	Capabilities    any        `json:"capabilities"`
	ClientInfo      ClientInfo `json:"clientInfo"`
	Roots           []Root     `json:"roots,omitempty"`
}

// Root represents a workspace root provided by the client.
type Root struct {
	URI  string `json:"uri"`
	Name string `json:"name,omitempty"`
}

// ClientInfo describes the connecting client.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult is the server's response to initialize.
type InitializeResult struct {
	ProtocolVersion string           `json:"protocolVersion"`
	Capabilities    ServerCapability `json:"capabilities"`
	ServerInfo      ServerInfo       `json:"serverInfo"`
}

// ServerCapability declares server capabilities.
type ServerCapability struct {
	Tools *ToolCapability `json:"tools,omitempty"`
}

// ToolCapability declares tool-related capabilities.
type ToolCapability struct {
	ListChanged bool `json:"listChanged"`
}

// ServerInfo identifies the server.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Tool represents an MCP tool definition.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// CallToolRequest is the params for tools/call.
type CallToolRequest struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// CallToolResult is the result of tools/call.
type CallToolResult struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ToolContent is a single content item in a tool result.
type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}
