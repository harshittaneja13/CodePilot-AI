package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog"
)

// StdioTransport manages a GitHub MCP server container over newline-delimited
// JSON-RPC. Requests are serialized because the review engine uses one shared
// server process and the server may emit protocol notifications between replies.
type StdioTransport struct {
	githubToken string
	mcpImage    string
	toolsets    string

	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    *bufio.Scanner
	responses chan *Response
	readErr   chan error

	mu     sync.Mutex
	nextID atomic.Int64
	closed atomic.Bool
	logger zerolog.Logger
}

func NewStdioTransport(githubToken, mcpImage, toolsets string, logger zerolog.Logger) *StdioTransport {
	return &StdioTransport{
		githubToken: githubToken,
		mcpImage:    mcpImage,
		toolsets:    toolsets,
		responses:   make(chan *Response, 16),
		readErr:     make(chan error, 1),
		logger:      logger.With().Str("component", "mcp-transport").Logger(),
	}
}

func (t *StdioTransport) Start(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.githubToken == "" {
		return fmt.Errorf("GitHub token is required to start MCP server")
	}

	t.logger.Info().Str("image", t.mcpImage).Str("toolsets", t.toolsets).Msg("starting MCP server container")
	t.cmd = exec.CommandContext(ctx, "docker", "run", "-i", "--rm",
		"-e", "GITHUB_PERSONAL_ACCESS_TOKEN", "-e", "GITHUB_TOOLSETS", t.mcpImage)
	t.cmd.Env = append(os.Environ(),
		"GITHUB_PERSONAL_ACCESS_TOKEN="+t.githubToken,
		"GITHUB_TOOLSETS="+t.toolsets,
	)

	var err error
	t.stdin, err = t.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("creating stdin pipe: %w", err)
	}
	stdoutPipe, err := t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating stdout pipe: %w", err)
	}
	t.stdout = bufio.NewScanner(stdoutPipe)
	t.stdout.Buffer(make([]byte, 0, 64*1024), 32*1024*1024)
	t.cmd.Stderr = os.Stderr

	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("starting Docker MCP container: %w", err)
	}
	go t.readLoop()
	t.logger.Info().Int("pid", t.cmd.Process.Pid).Msg("MCP server container started")
	return nil
}

func (t *StdioTransport) readLoop() {
	for t.stdout.Scan() {
		line := append([]byte(nil), t.stdout.Bytes()...)
		if len(line) == 0 {
			continue
		}
		var response Response
		if err := json.Unmarshal(line, &response); err != nil {
			t.logger.Debug().Str("line", string(line)).Msg("ignoring non-response MCP output")
			continue
		}
		if response.ID == nil {
			continue
		}
		t.responses <- &response
	}
	if err := t.stdout.Err(); err != nil {
		t.readErr <- fmt.Errorf("reading MCP stdout: %w", err)
	} else {
		t.readErr <- fmt.Errorf("MCP stdout closed unexpectedly")
	}
}

func (t *StdioTransport) NextID() int64 { return t.nextID.Add(1) }

func (t *StdioTransport) Send(ctx context.Context, req *Request) (*Response, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed.Load() {
		return nil, fmt.Errorf("transport is closed")
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling MCP request: %w", err)
	}
	if _, err := t.stdin.Write(append(data, '\n')); err != nil {
		return nil, fmt.Errorf("writing MCP request: %w", err)
	}
	if req.ID == nil {
		return nil, nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("waiting for MCP response: %w", ctx.Err())
		case err := <-t.readErr:
			return nil, err
		case response := <-t.responses:
			if fmt.Sprint(response.ID) != fmt.Sprint(req.ID) {
				t.logger.Warn().Interface("expected_id", req.ID).Interface("response_id", response.ID).Msg("ignoring mismatched MCP response")
				continue
			}
			return response, nil
		}
	}
}

func (t *StdioTransport) SendNotification(ctx context.Context, method string, params interface{}) error {
	var rawParams json.RawMessage
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("marshaling MCP notification: %w", err)
		}
		rawParams = data
	}
	_, err := t.Send(ctx, &Request{JSONRPC: "2.0", Method: method, Params: rawParams})
	return err
}

func (t *StdioTransport) Close() error {
	if t.closed.Swap(true) {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	var firstErr error
	if t.stdin != nil {
		if err := t.stdin.Close(); err != nil {
			firstErr = err
		}
	}
	if t.cmd != nil && t.cmd.Process != nil {
		if err := t.cmd.Process.Kill(); err != nil && firstErr == nil {
			firstErr = err
		}
		_ = t.cmd.Wait()
	}
	return firstErr
}
