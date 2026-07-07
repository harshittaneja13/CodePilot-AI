// Package mcpserver implements a Model Context Protocol (MCP) server that exposes
// CodePilot's own capabilities — review history and semantic code retrieval — as MCP
// tools. It speaks newline-delimited JSON-RPC 2.0 over an io.Reader/io.Writer (stdin/
// stdout in production), so any MCP host (Claude Desktop, IDEs) can call these tools.
//
// This complements internal/mcp (which is an MCP *client* to GitHub's server): the
// project is now both an MCP client and an MCP server.
package mcpserver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/rs/zerolog"

	"github.com/codepilot-ai/codepilot-ai/internal/mcp"
)

// protocolVersion is the MCP protocol revision this server implements.
const protocolVersion = "2024-11-05"

// ToolHandler executes a tool call and returns its text output, or an error (which the
// server reports to the caller as an isError tool result rather than a protocol error).
type ToolHandler func(ctx context.Context, args map[string]interface{}) (string, error)

type tool struct {
	name        string
	description string
	inputSchema map[string]interface{}
	handler     ToolHandler
}

// Server is an MCP server. Register tools, then call Serve.
type Server struct {
	name    string
	version string
	tools   []tool
	logger  zerolog.Logger
}

// New creates an MCP server identified by name/version. logger should write to stderr
// (never the stdout used for protocol messages); zerolog.Nop() is fine in tests.
func New(name, version string, logger zerolog.Logger) *Server {
	return &Server{
		name:    name,
		version: version,
		logger:  logger.With().Str("component", "mcp-server").Logger(),
	}
}

// Register adds a tool with its JSON-schema input definition and handler.
func (s *Server) Register(name, description string, inputSchema map[string]interface{}, handler ToolHandler) {
	s.tools = append(s.tools, tool{name: name, description: description, inputSchema: inputSchema, handler: handler})
}

// Serve reads newline-delimited JSON-RPC requests from r and writes responses to w
// until r is exhausted or ctx is cancelled. Notifications (no id) receive no response.
func (s *Server) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	enc := json.NewEncoder(w) // Encode appends '\n', giving newline-delimited framing

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req mcp.Request
		if err := json.Unmarshal(line, &req); err != nil {
			_ = enc.Encode(s.errorResp(nil, -32700, "parse error"))
			continue
		}

		resp := s.handle(ctx, &req)
		if req.ID == nil {
			// JSON-RPC notification (e.g. notifications/initialized) — no reply.
			continue
		}
		if err := enc.Encode(resp); err != nil {
			return fmt.Errorf("writing response: %w", err)
		}
	}
	return scanner.Err()
}

func (s *Server) handle(ctx context.Context, req *mcp.Request) *mcp.Response {
	switch req.Method {
	case "initialize":
		return s.result(req.ID, mcp.InitializeResult{
			ProtocolVersion: protocolVersion,
			Capabilities:    mcp.ServerCapabilities{Tools: &struct{}{}},
			ServerInfo:      mcp.ServerInfo{Name: s.name, Version: s.version},
		})
	case "ping":
		return s.result(req.ID, map[string]interface{}{})
	case "tools/list":
		return s.result(req.ID, s.listTools())
	case "tools/call":
		return s.callTool(ctx, req)
	case "notifications/initialized", "initialized":
		return nil // notification; Serve skips sending because ID is nil
	default:
		return s.errorResp(req.ID, -32601, "method not found: "+req.Method)
	}
}

func (s *Server) listTools() mcp.ListToolsResult {
	out := mcp.ListToolsResult{Tools: make([]mcp.Tool, 0, len(s.tools))}
	for _, t := range s.tools {
		schema, err := json.Marshal(t.inputSchema)
		if err != nil {
			s.logger.Error().Err(err).Str("tool", t.name).Msg("failed to marshal tool schema")
			continue
		}
		out.Tools = append(out.Tools, mcp.Tool{Name: t.name, Description: t.description, InputSchema: schema})
	}
	return out
}

func (s *Server) callTool(ctx context.Context, req *mcp.Request) *mcp.Response {
	var params mcp.CallToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.errorResp(req.ID, -32602, "invalid tool call params")
	}
	for _, t := range s.tools {
		if t.name != params.Name {
			continue
		}
		text, err := t.handler(ctx, params.Arguments)
		if err != nil {
			// Tool-level failures are returned as isError results, not JSON-RPC errors,
			// so the host can show the message to the model/user.
			return s.result(req.ID, toolResult("Error: "+err.Error(), true))
		}
		return s.result(req.ID, toolResult(text, false))
	}
	return s.result(req.ID, toolResult("unknown tool: "+params.Name, true))
}

func toolResult(text string, isErr bool) mcp.CallToolResult {
	return mcp.CallToolResult{Content: []mcp.Content{{Type: "text", Text: text}}, IsError: isErr}
}

func (s *Server) result(id interface{}, v interface{}) *mcp.Response {
	raw, err := json.Marshal(v)
	if err != nil {
		return s.errorResp(id, -32603, "internal error: "+err.Error())
	}
	return &mcp.Response{JSONRPC: "2.0", ID: id, Result: raw}
}

func (s *Server) errorResp(id interface{}, code int, msg string) *mcp.Response {
	return &mcp.Response{JSONRPC: "2.0", ID: id, Error: &mcp.RPCError{Code: code, Message: msg}}
}
