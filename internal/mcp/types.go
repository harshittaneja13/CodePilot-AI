package mcp

import "encoding/json"

// --- JSON-RPC 2.0 Base Types ---

// Request represents a JSON-RPC 2.0 request message.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents a JSON-RPC 2.0 response message.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Error implements the error interface for RPCError.
func (e *RPCError) Error() string {
	return e.Message
}

// --- MCP Protocol Types ---

// ClientInfo identifies this MCP client to the server.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ServerInfo is returned by the MCP server during initialization.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeParams are sent with the initialize request.
type InitializeParams struct {
	ProtocolVersion string     `json:"protocolVersion"`
	Capabilities    struct{}   `json:"capabilities"`
	ClientInfo      ClientInfo `json:"clientInfo"`
}

// ServerCapabilities describes what the MCP server supports.
type ServerCapabilities struct {
	Tools *struct{} `json:"tools,omitempty"`
}

// InitializeResult is the response from the initialize method.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

// Tool describes an available MCP tool.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// ListToolsResult is the response from tools/list.
type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

// CallToolParams are sent with the tools/call request.
type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// Content represents a piece of content in a tool result.
type Content struct {
	Type     string `json:"type"`               // "text" or "image"
	Text     string `json:"text,omitempty"`      // present when type == "text"
	Data     string `json:"data,omitempty"`      // base64-encoded, present when type == "image"
	MimeType string `json:"mimeType,omitempty"`  // present when type == "image"
}

// CallToolResult is the response from tools/call.
type CallToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// ReviewCommentInput represents a comment to attach to a pull request review.
type ReviewCommentInput struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Body string `json:"body"`
}
