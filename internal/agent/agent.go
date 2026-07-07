// Package agent implements an autonomous, tool-using PR-review loop. Instead of a
// fixed prompt pipeline, the model decides which tools to call (read a file, search
// code, read a diff) before submitting findings. The review engine falls back to the
// deterministic pipeline when a model does not support tool calling.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/codepilot-ai/codepilot-ai/internal/llm"
	"github.com/codepilot-ai/codepilot-ai/internal/models"
	"github.com/codepilot-ai/codepilot-ai/internal/rag"
)

const (
	// defaultMaxSteps bounds how many tool-use turns the agent may take.
	defaultMaxSteps = 8
	// maxToolResultChars caps a single tool result to control token growth.
	maxToolResultChars = 4000
	// maxInitialPatchChars caps each diff shown in the opening context message.
	maxInitialPatchChars = 2000
)

// MCPTools is the subset of the MCP client the agent needs (an interface so tests
// can supply a fake).
type MCPTools interface {
	GetFileContents(ctx context.Context, owner, repo, path, ref string) (string, error)
	SearchCode(ctx context.Context, query string) ([]map[string]interface{}, error)
}

// ToolLLM is the LLM capability the agent needs (an interface so tests can supply a fake).
type ToolLLM interface {
	ChatWithTools(ctx context.Context, messages []llm.AgentMessage, tools []llm.ToolDefinition, model string) (*llm.ToolChatResponse, error)
}

// Input holds everything the agent needs to review one PR.
type Input struct {
	Owner       string
	Repo        string
	HeadRef     string // commit SHA used when reading file contents
	PR          *models.PullRequest
	Files       []llm.FileContext // changed files with patches (already triaged)
	RepoContext string
	Model       string // per-repo model override; "" uses the client default
}

// TraceStep records one tool invocation for the dashboard/observability timeline.
type TraceStep struct {
	Step       int    `json:"step"`
	Tool       string `json:"tool"`
	Args       string `json:"args"`
	Result     string `json:"result"`
	DurationMs int64  `json:"duration_ms"`
}

// Result is the agent's final output.
type Result struct {
	Summary      string
	Findings     []llm.ReviewFinding
	TokensUsed   int // total (input + output) across all tool-use turns
	InputTokens  int
	OutputTokens int
	Steps        int
	Trace        []TraceStep
}

// Agent runs a bounded tool-use loop to review a PR.
type Agent struct {
	llm       ToolLLM
	mcp       MCPTools
	retriever rag.Retriever // optional; when set, the retrieve_context tool is offered
	maxSteps  int
	logger    zerolog.Logger
}

// New creates an Agent. logger may be zerolog.Nop() in tests.
func New(toolLLM ToolLLM, mcpTools MCPTools, logger zerolog.Logger) *Agent {
	return &Agent{
		llm:      toolLLM,
		mcp:      mcpTools,
		maxSteps: defaultMaxSteps,
		logger:   logger.With().Str("component", "review-agent").Logger(),
	}
}

// SetMaxSteps overrides the tool-use step budget (primarily for tests).
func (a *Agent) SetMaxSteps(n int) {
	if n > 0 {
		a.maxSteps = n
	}
}

// SetRetriever enables the retrieve_context tool backed by the given RAG retriever.
func (a *Agent) SetRetriever(r rag.Retriever) { a.retriever = r }

// Run executes the tool-use loop and returns the review. It returns an error if the
// model never submits findings (e.g. it lacks tool support or misbehaves); the caller
// should then fall back to the deterministic pipeline.
func (a *Agent) Run(ctx context.Context, in Input) (*Result, error) {
	patches := make(map[string]string, len(in.Files))
	for _, f := range in.Files {
		patches[f.Path] = f.Patch
	}

	messages := []llm.AgentMessage{
		{Role: "system", Content: agentSystemPrompt},
		{Role: "user", Content: buildInitialContext(in)},
	}
	tools := toolDefinitions(a.retriever != nil)

	res := &Result{}
	seen := make(map[string]bool) // dedup identical tool calls

	for step := 1; step <= a.maxSteps; step++ {
		resp, err := a.llm.ChatWithTools(ctx, messages, tools, in.Model)
		if err != nil {
			return nil, fmt.Errorf("agent step %d: %w", step, err)
		}
		res.TokensUsed += resp.TokensUsed
		res.InputTokens += resp.InputTokens
		res.OutputTokens += resp.OutputTokens
		res.Steps = step

		// No tool calls: the model produced a final message. Accept it only if it
		// parses as a review; otherwise signal the caller to fall back.
		if len(resp.ToolCalls) == 0 {
			if parsed, perr := llm.ParseReviewResponse(resp.Content); perr == nil && parsed.Summary != "" {
				res.Summary = parsed.Summary
				res.Findings = parsed.Findings
				return res, nil
			}
			return nil, fmt.Errorf("agent stopped at step %d without calling %s", step, toolSubmitFindings)
		}

		// Record the assistant's tool-call turn so the model sees its own calls.
		messages = append(messages, llm.AgentMessage{Role: "assistant", Content: resp.Content, ToolCalls: resp.ToolCalls})

		for _, tc := range resp.ToolCalls {
			if tc.Name == toolSubmitFindings {
				summary, findings, perr := parseSubmitFindings(tc.Arguments)
				if perr != nil {
					// Let the model retry with a corrected call.
					messages = append(messages, toolResult(tc.ID, "Error parsing findings: "+perr.Error()+". Re-call submit_findings with valid JSON."))
					res.Trace = append(res.Trace, TraceStep{Step: step, Tool: tc.Name, Args: truncate(tc.Arguments, 300), Result: "parse error"})
					continue
				}
				res.Summary = summary
				res.Findings = findings
				res.Trace = append(res.Trace, TraceStep{Step: step, Tool: tc.Name, Args: truncate(tc.Arguments, 300), Result: fmt.Sprintf("submitted %d findings", len(findings))})
				return res, nil
			}

			start := time.Now()
			out := a.execTool(ctx, in, patches, tc, seen)
			res.Trace = append(res.Trace, TraceStep{
				Step:       step,
				Tool:       tc.Name,
				Args:       truncate(tc.Arguments, 300),
				Result:     truncate(out, 300),
				DurationMs: time.Since(start).Milliseconds(),
			})
			messages = append(messages, toolResult(tc.ID, out))
		}
	}

	return nil, fmt.Errorf("agent exceeded max steps (%d) without submitting findings", a.maxSteps)
}

// execTool runs a single non-terminal tool call and returns a string result to feed
// back to the model. Errors are returned as messages (not Go errors) so the model can
// recover. Identical repeat calls are short-circuited to avoid wasted loops/tokens.
func (a *Agent) execTool(ctx context.Context, in Input, patches map[string]string, tc llm.ToolCall, seen map[string]bool) string {
	key := tc.Name + ":" + tc.Arguments
	if seen[key] {
		return "You already called this tool with the same arguments; reuse the earlier result."
	}
	seen[key] = true

	switch tc.Name {
	case toolGetFileContents:
		path, ok := stringArg(tc.Arguments, "path")
		if !ok {
			return `Error: invalid arguments; expected {"path": "..."}`
		}
		content, err := a.mcp.GetFileContents(ctx, in.Owner, in.Repo, path, in.HeadRef)
		if err != nil {
			return "Error reading file: " + err.Error()
		}
		return truncate(content, maxToolResultChars)

	case toolSearchCode:
		query, ok := stringArg(tc.Arguments, "query")
		if !ok {
			return `Error: invalid arguments; expected {"query": "..."}`
		}
		results, err := a.mcp.SearchCode(ctx, query)
		if err != nil {
			return "Error searching code: " + err.Error()
		}
		return truncate(formatSearchResults(results), maxToolResultChars)

	case toolGetFileDiff:
		path, ok := stringArg(tc.Arguments, "path")
		if !ok {
			return `Error: invalid arguments; expected {"path": "..."}`
		}
		patch, ok := patches[path]
		if !ok {
			return fmt.Sprintf("No diff for %q. Changed files: %s", path, strings.Join(changedPaths(patches), ", "))
		}
		return truncate(patch, maxToolResultChars)

	case toolRetrieveContext:
		if a.retriever == nil {
			return "Error: retrieve_context is not available."
		}
		query, ok := stringArg(tc.Arguments, "query")
		if !ok {
			return `Error: invalid arguments; expected {"query": "..."}`
		}
		chunks, err := a.retriever.Retrieve(ctx, in.Owner+"/"+in.Repo, query, 5)
		if err != nil {
			return "Error retrieving context: " + err.Error()
		}
		return truncate(formatChunks(chunks), maxToolResultChars)

	default:
		return "Error: unknown tool " + tc.Name
	}
}

// stringArg extracts a required string field from raw JSON tool arguments.
func stringArg(rawArgs, field string) (string, bool) {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(rawArgs), &m); err != nil {
		return "", false
	}
	v, ok := m[field].(string)
	if !ok || v == "" {
		return "", false
	}
	return v, true
}

// parseSubmitFindings decodes the terminal submit_findings arguments.
func parseSubmitFindings(rawArgs string) (string, []llm.ReviewFinding, error) {
	var out struct {
		Summary  string              `json:"summary"`
		Findings []llm.ReviewFinding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(rawArgs), &out); err != nil {
		return "", nil, err
	}
	if out.Findings == nil {
		out.Findings = []llm.ReviewFinding{}
	}
	return out.Summary, out.Findings, nil
}

// changedPaths returns the keys of the patch map for error messages.
func changedPaths(patches map[string]string) []string {
	paths := make([]string, 0, len(patches))
	for p := range patches {
		paths = append(paths, p)
	}
	return paths
}
