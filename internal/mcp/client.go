package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/rs/zerolog"
)

const (
	mcpProtocolVersion = "2024-11-05"
	clientName         = "codepilot-ai"
	clientVersion      = "1.0.0"
)

// Client provides a high-level interface for communicating with the GitHub MCP server.
type Client struct {
	transport   *StdioTransport
	initialized bool
	tools       []Tool
	toolNames   map[string]struct{}
	logger      zerolog.Logger
}

// NewClient creates a new MCP Client.
func NewClient(githubToken string, mcpImage string, toolsets string, logger zerolog.Logger) *Client {
	return &Client{
		transport: NewStdioTransport(githubToken, mcpImage, toolsets, logger),
		toolNames: make(map[string]struct{}),
		logger:    logger.With().Str("component", "mcp-client").Logger(),
	}
}

// Connect starts the transport, performs the MCP initialize handshake,
// sends the initialized notification, and discovers available tools.
func (c *Client) Connect(ctx context.Context) error {
	c.logger.Info().Msg("connecting to MCP server")

	if err := c.transport.Start(ctx); err != nil {
		return fmt.Errorf("starting transport: %w", err)
	}

	// Step 1: Send initialize request.
	initParams := InitializeParams{
		ProtocolVersion: mcpProtocolVersion,
		ClientInfo: ClientInfo{
			Name:    clientName,
			Version: clientVersion,
		},
	}
	paramsData, err := json.Marshal(initParams)
	if err != nil {
		return fmt.Errorf("marshaling initialize params: %w", err)
	}

	req := &Request{
		JSONRPC: "2.0",
		ID:      c.transport.NextID(),
		Method:  "initialize",
		Params:  paramsData,
	}

	resp, err := c.transport.Send(ctx, req)
	if err != nil {
		return fmt.Errorf("sending initialize request: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("initialize error: %s (code %d)", resp.Error.Message, resp.Error.Code)
	}

	var initResult InitializeResult
	if err := json.Unmarshal(resp.Result, &initResult); err != nil {
		return fmt.Errorf("parsing initialize result: %w", err)
	}

	c.logger.Info().
		Str("server", initResult.ServerInfo.Name).
		Str("version", initResult.ServerInfo.Version).
		Str("protocol", initResult.ProtocolVersion).
		Msg("MCP server initialized")

	// Step 2: Send initialized notification.
	if err := c.transport.SendNotification(ctx, "notifications/initialized", nil); err != nil {
		return fmt.Errorf("sending initialized notification: %w", err)
	}

	c.initialized = true

	// Step 3: Discover available tools.
	tools, err := c.ListTools(ctx)
	if err != nil {
		c.logger.Warn().Err(err).Msg("failed to list tools during connect; continuing")
	} else {
		c.tools = tools
		for _, tool := range tools {
			c.toolNames[tool.Name] = struct{}{}
		}
		c.logger.Info().Int("count", len(tools)).Msg("discovered MCP tools")
	}

	return nil
}

// ListTools retrieves the list of available tools from the MCP server.
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized; call Connect first")
	}

	req := &Request{
		JSONRPC: "2.0",
		ID:      c.transport.NextID(),
		Method:  "tools/list",
	}

	resp, err := c.transport.Send(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("listing tools: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("tools/list error: %s", resp.Error.Message)
	}

	var result ListToolsResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parsing tools/list result: %w", err)
	}

	return result.Tools, nil
}

// CallTool invokes a named tool on the MCP server with the given arguments.
func (c *Client) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (*CallToolResult, error) {
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized; call Connect first")
	}

	params := CallToolParams{
		Name:      toolName,
		Arguments: args,
	}
	paramsData, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshaling tool call params: %w", err)
	}

	req := &Request{
		JSONRPC: "2.0",
		ID:      c.transport.NextID(),
		Method:  "tools/call",
		Params:  paramsData,
	}

	c.logger.Debug().
		Str("tool", toolName).
		Interface("args", args).
		Msg("calling MCP tool")

	resp, err := c.transport.Send(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("calling tool %q: %w", toolName, err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("tool %q error: %s (code %d)", toolName, resp.Error.Message, resp.Error.Code)
	}

	var result CallToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parsing tool %q result: %w", toolName, err)
	}

	if result.IsError {
		text := ""
		for _, c := range result.Content {
			if c.Type == "text" {
				text += c.Text
			}
		}
		return nil, fmt.Errorf("tool %q returned error: %s", toolName, text)
	}

	return &result, nil
}

// GetTextContent extracts all text content from a CallToolResult.
func GetTextContent(result *CallToolResult) string {
	var text string
	for _, c := range result.Content {
		if c.Type == "text" {
			text += c.Text
		}
	}
	return text
}

func (c *Client) hasTool(name string) bool {
	_, ok := c.toolNames[name]
	return ok
}

// --- GitHub-specific helper methods ---

// GetPullRequest fetches pull request details via the MCP server.
func (c *Client) GetPullRequest(ctx context.Context, owner, repo string, number int) (map[string]interface{}, error) {
	toolName := "get_pull_request"
	args := map[string]interface{}{"owner": owner, "repo": repo, "pull_number": number}
	if c.hasTool("pull_request_read") {
		toolName = "pull_request_read"
		args = map[string]interface{}{"method": "get", "owner": owner, "repo": repo, "pullNumber": number}
	}
	result, err := c.CallTool(ctx, toolName, args)
	if err != nil {
		return nil, fmt.Errorf("getting PR #%d: %w", number, err)
	}

	text := GetTextContent(result)
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		return nil, fmt.Errorf("parsing PR response: %w", err)
	}

	return data, nil
}

// FetchPRFiles fetches the list of changed files for a PR from the GitHub REST API.
func (c *Client) FetchPRFiles(ctx context.Context, owner, repo string, prNumber int) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d/files?per_page=100", owner, repo, prNumber)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building PR files request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.transport.githubToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching PR files: %w", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API %d for PR files: %s", resp.StatusCode, string(body))
	}
	var files []map[string]interface{}
	if err := json.Unmarshal(body, &files); err != nil {
		return nil, fmt.Errorf("parsing PR files: %w", err)
	}
	return files, nil
}

// ListPullRequestFiles lists files changed in a pull request.
func (c *Client) ListPullRequestFiles(ctx context.Context, owner, repo string, number int) ([]map[string]interface{}, error) {
	toolName := "list_pull_request_files"
	args := map[string]interface{}{"owner": owner, "repo": repo, "pull_number": number}
	if c.hasTool("pull_request_read") {
		toolName = "pull_request_read"
		args = map[string]interface{}{"method": "get_files", "owner": owner, "repo": repo, "pullNumber": number, "perPage": 100}
	}
	result, err := c.CallTool(ctx, toolName, args)
	if err != nil {
		return nil, fmt.Errorf("listing PR #%d files: %w", number, err)
	}

	text := GetTextContent(result)
	files, err := decodeObjectList(text, "files")
	if err != nil {
		return nil, fmt.Errorf("parsing PR files response: %w", err)
	}

	return files, nil
}

// GetPullRequestDiff returns the unified diff for a pull request.
func (c *Client) GetPullRequestDiff(ctx context.Context, owner, repo string, number int) (string, error) {
	if !c.hasTool("pull_request_read") {
		return "", nil
	}
	result, err := c.CallTool(ctx, "pull_request_read", map[string]interface{}{
		"method": "get_diff", "owner": owner, "repo": repo, "pullNumber": number,
	})
	if err != nil {
		return "", err
	}
	return GetTextContent(result), nil
}

// GetPullRequestCommits returns the commits included in a pull request.
func (c *Client) GetPullRequestCommits(ctx context.Context, owner, repo string, number int) ([]map[string]interface{}, error) {
	if !c.hasTool("pull_request_read") {
		return []map[string]interface{}{}, nil
	}
	result, err := c.CallTool(ctx, "pull_request_read", map[string]interface{}{
		"method": "get_commits", "owner": owner, "repo": repo, "pullNumber": number, "perPage": 100,
	})
	if err != nil {
		return nil, err
	}
	return decodeObjectList(GetTextContent(result), "commits")
}

// GetPullRequestReviewComments returns existing review threads used for duplicate suppression.
func (c *Client) GetPullRequestReviewComments(ctx context.Context, owner, repo string, number int) ([]map[string]interface{}, error) {
	if !c.hasTool("pull_request_read") {
		return []map[string]interface{}{}, nil
	}
	result, err := c.CallTool(ctx, "pull_request_read", map[string]interface{}{
		"method": "get_review_comments", "owner": owner, "repo": repo, "pullNumber": number, "perPage": 100,
	})
	if err != nil {
		return nil, err
	}
	return decodeObjectList(GetTextContent(result), "threads")
}

// GetPullRequestChecks returns current CI check runs for the PR head commit.
func (c *Client) GetPullRequestChecks(ctx context.Context, owner, repo string, number int) ([]map[string]interface{}, error) {
	if !c.hasTool("pull_request_read") {
		return []map[string]interface{}{}, nil
	}
	result, err := c.CallTool(ctx, "pull_request_read", map[string]interface{}{
		"method": "get_check_runs", "owner": owner, "repo": repo, "pullNumber": number, "perPage": 100,
	})
	if err != nil {
		return nil, err
	}
	return decodeObjectList(GetTextContent(result), "check_runs")
}

// GetRepository fetches repository metadata directly from the GitHub REST API.
// The GitHub MCP server does not expose a get_repository tool, so we call
// the REST API with the same token the MCP transport holds.
func (c *Client) GetRepository(ctx context.Context, owner, repo string) (map[string]interface{}, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building GitHub API request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.transport.githubToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling GitHub API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading GitHub API response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(body))
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("parsing repository response: %w", err)
	}
	return data, nil
}

// CreateWebhook registers a webhook on the GitHub repository pointing to our server.
// It returns the webhook ID assigned by GitHub (stored in the repository record).
func (c *Client) CreateWebhook(ctx context.Context, owner, repo, webhookURL, secret string) (int64, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/hooks", owner, repo)
	payload, _ := json.Marshal(map[string]interface{}{
		"name":   "web",
		"active": true,
		"events": []string{"pull_request"},
		"config": map[string]interface{}{
			"url":          webhookURL,
			"content_type": "json",
			"secret":       secret,
			"insecure_ssl": "0",
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return 0, fmt.Errorf("building create-webhook request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.transport.githubToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("calling create-webhook API: %w", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return 0, fmt.Errorf("GitHub API %d creating webhook: %s", resp.StatusCode, string(body))
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("parsing webhook response: %w", err)
	}
	id, _ := result["id"].(float64)
	return int64(id), nil
}

// SyncPullRequests fetches open (and recently closed) pull requests from the
// GitHub REST API and returns them as raw maps for the caller to upsert.
func (c *Client) SyncPullRequests(ctx context.Context, owner, repo string) ([]map[string]interface{}, error) {
	var all []map[string]interface{}
	for _, state := range []string{"open", "closed"} {
		url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?state=%s&per_page=50&sort=updated&direction=desc", owner, repo, state)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("building pulls request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.transport.githubToken)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetching %s pulls: %w", state, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("GitHub API %d for %s pulls: %s", resp.StatusCode, state, string(body))
		}
		var prs []map[string]interface{}
		if err := json.Unmarshal(body, &prs); err != nil {
			return nil, fmt.Errorf("parsing %s pulls: %w", state, err)
		}
		all = append(all, prs...)
	}
	return all, nil
}

// FetchPRStats fetches additions, deletions, and changed_files for a single PR
// from the GitHub individual PR endpoint (the list endpoint omits diff stats).
func (c *Client) FetchPRStats(ctx context.Context, owner, repo string, prNumber int) (additions, deletions, changedFiles int, err error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repo, prNumber)
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if reqErr != nil {
		return 0, 0, 0, fmt.Errorf("building PR stats request: %w", reqErr)
	}
	req.Header.Set("Authorization", "Bearer "+c.transport.githubToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, doErr := http.DefaultClient.Do(req)
	if doErr != nil {
		return 0, 0, 0, fmt.Errorf("fetching PR stats: %w", doErr)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, 0, 0, fmt.Errorf("GitHub API %d for PR stats: %s", resp.StatusCode, string(body))
	}
	var pr map[string]interface{}
	if err := json.Unmarshal(body, &pr); err != nil {
		return 0, 0, 0, fmt.Errorf("parsing PR stats: %w", err)
	}
	if v, ok := pr["additions"].(float64); ok {
		additions = int(v)
	}
	if v, ok := pr["deletions"].(float64); ok {
		deletions = int(v)
	}
	if v, ok := pr["changed_files"].(float64); ok {
		changedFiles = int(v)
	}
	return additions, deletions, changedFiles, nil
}

// GetFileContents retrieves file contents from a repository at a specific ref.
func (c *Client) GetFileContents(ctx context.Context, owner, repo, path, ref string) (string, error) {
	args := map[string]interface{}{
		"owner": owner,
		"repo":  repo,
		"path":  path,
	}
	if ref != "" {
		args["ref"] = ref
	}

	result, err := c.CallTool(ctx, "get_file_contents", args)
	if err != nil {
		return "", fmt.Errorf("getting file %q at ref %q: %w", path, ref, err)
	}

	return decodeFileContent(GetTextContent(result)), nil
}

func decodeFileContent(text string) string {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		return text
	}
	content, ok := data["content"].(string)
	if !ok {
		return text
	}
	if encoding, _ := data["encoding"].(string); encoding == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(content)
		if err == nil {
			return string(decoded)
		}
	}
	return content
}

// CreatePullRequestReview submits a review on a pull request with optional inline comments.
func (c *Client) CreatePullRequestReview(ctx context.Context, owner, repo string, number int, body, event string, comments []ReviewCommentInput) error {
	if c.hasTool("pull_request_review_write") && c.hasTool("add_comment_to_pending_review") {
		base := map[string]interface{}{"owner": owner, "repo": repo, "pullNumber": number}
		createArgs := cloneArgs(base)
		createArgs["method"] = "create"
		if _, err := c.CallTool(ctx, "pull_request_review_write", createArgs); err != nil {
			return fmt.Errorf("creating pending review: %w", err)
		}
		for _, comment := range comments {
			args := cloneArgs(base)
			args["path"] = comment.Path
			args["line"] = comment.Line
			args["side"] = "RIGHT"
			args["subjectType"] = "LINE"
			args["body"] = comment.Body
			if _, err := c.CallTool(ctx, "add_comment_to_pending_review", args); err != nil {
				cleanup := cloneArgs(base)
				cleanup["method"] = "delete_pending"
				_, _ = c.CallTool(context.Background(), "pull_request_review_write", cleanup)
				return fmt.Errorf("adding inline review comment for %s:%d: %w", comment.Path, comment.Line, err)
			}
		}
		submitArgs := cloneArgs(base)
		submitArgs["method"] = "submit_pending"
		submitArgs["body"] = body
		submitArgs["event"] = event
		if _, err := c.CallTool(ctx, "pull_request_review_write", submitArgs); err != nil {
			return fmt.Errorf("submitting pending review: %w", err)
		}
		return nil
	}

	args := map[string]interface{}{
		"owner":       owner,
		"repo":        repo,
		"pull_number": number,
		"body":        body,
		"event":       event,
	}

	if len(comments) > 0 {
		args["comments"] = comments
	}

	_, err := c.CallTool(ctx, "create_pull_request_review", args)
	if err != nil {
		return fmt.Errorf("creating review on PR #%d: %w", number, err)
	}

	return nil
}

func cloneArgs(source map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(source)+4)
	for key, value := range source {
		result[key] = value
	}
	return result
}

func decodeObjectList(text, preferredKey string) ([]map[string]interface{}, error) {
	var direct []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &direct); err == nil {
		return direct, nil
	}
	var wrapped map[string]interface{}
	if err := json.Unmarshal([]byte(text), &wrapped); err != nil {
		return nil, err
	}
	keys := []string{preferredKey, "items", "nodes", "data"}
	for _, key := range keys {
		items, ok := wrapped[key].([]interface{})
		if !ok {
			continue
		}
		result := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			if object, ok := item.(map[string]interface{}); ok {
				result = append(result, object)
			}
		}
		return result, nil
	}
	return []map[string]interface{}{}, nil
}

// SearchCode performs a code search query on GitHub.
func (c *Client) SearchCode(ctx context.Context, query string) ([]map[string]interface{}, error) {
	result, err := c.CallTool(ctx, "search_code", map[string]interface{}{
		"q": query,
	})
	if err != nil {
		return nil, fmt.Errorf("searching code: %w", err)
	}

	text := GetTextContent(result)
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		return nil, fmt.Errorf("parsing search response: %w", err)
	}

	items, ok := data["items"].([]interface{})
	if !ok {
		return nil, nil
	}

	results := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			results = append(results, m)
		}
	}

	return results, nil
}

// Close shuts down the MCP client and its transport.
func (c *Client) Close() error {
	c.logger.Info().Msg("closing MCP client")
	c.initialized = false
	return c.transport.Close()
}
