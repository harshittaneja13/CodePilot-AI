package agent

import (
	"fmt"
	"strings"

	"github.com/codepilot-ai/codepilot-ai/internal/llm"
	"github.com/codepilot-ai/codepilot-ai/internal/rag"
)

// agentSystemPrompt instructs the model to review a PR agentically: gather whatever
// context it needs via tools, then submit findings once. It intentionally mirrors the
// severity/confidence conventions used by the fixed pipeline (see internal/llm/prompt.go)
// so findings are consistent regardless of which path produced them.
const agentSystemPrompt = `You are an expert code reviewer operating as an autonomous agent on a GitHub pull request.

You have tools to explore the repository. Use them deliberately:
- get_file_diff(path): read the diff of a changed file.
- get_file_contents(path): read a full file at the PR head — use it to see code AROUND a change, a function's definition, or how something is used.
- search_code(query): find where a symbol/function is defined or called.
- submit_findings(summary, findings): finish the review. Call this EXACTLY ONCE.

Guidance:
- Prefer reading surrounding context before flagging an issue; many "bugs" disappear once you see the full function.
- Do not re-request the same file/diff you already have.
- Investigate only what matters. A small, clear PR may need no tool calls before submitting.
- Report only genuine, actionable issues. Set confidence honestly (1.0=certain, 0.6=probable, 0.2=likely false positive).
- Severity: critical=security/data-loss/crash, high=logic error/resource leak/missing error handling, medium=performance/validation/code smell, low=readability/naming.
- Only reference line numbers that appear in a diff.

When your analysis is complete, call submit_findings. If there are no genuine issues, submit an empty findings array with a brief positive summary.`

// buildInitialContext produces the first user message: PR metadata, optional
// repository context, and the changed-file diffs the agent starts from. The agent
// may fetch more via tools, so per-file patches are bounded here to control tokens.
func buildInitialContext(in Input) string {
	var sb strings.Builder

	pr := in.PR
	sb.WriteString(fmt.Sprintf("## Pull Request: %s\n\n", pr.Title))
	sb.WriteString(fmt.Sprintf("**Author:** %s | **Branch:** %s → %s | **Changes:** +%d/-%d across %d files\n\n",
		pr.Author, pr.HeadBranchStr, pr.BaseBranchStr, pr.Additions, pr.Deletions, pr.ChangedFiles))

	if pr.BodyStr != "" {
		body := pr.BodyStr
		if len(body) > 800 {
			body = body[:800] + "..."
		}
		sb.WriteString("<pr_description>\n" + body + "\n</pr_description>\n\n")
	}

	if in.RepoContext != "" {
		rc := in.RepoContext
		if len(rc) > 800 {
			rc = rc[:800] + "\n... (truncated)"
		}
		sb.WriteString("<repository_context>\n" + rc + "\n</repository_context>\n\n")
	}

	sb.WriteString("### Changed Files\n\n")
	if len(in.Files) == 0 {
		sb.WriteString("(No reviewable file diffs were provided; use tools to inspect the PR.)\n")
		return sb.String()
	}
	for _, f := range in.Files {
		sb.WriteString(fmt.Sprintf("#### %s (%s)\n<diff>\n", f.Path, f.Language))
		patch := f.Patch
		if len(patch) > maxInitialPatchChars {
			patch = patch[:maxInitialPatchChars] + "\n... (truncated — use get_file_diff for the full patch)"
		}
		sb.WriteString(patch + "\n</diff>\n\n")
	}
	return sb.String()
}

// toolResult builds a tool-result message answering a specific tool call.
func toolResult(callID, content string) llm.AgentMessage {
	return llm.AgentMessage{Role: "tool", ToolCallID: callID, Content: content}
}

// formatSearchResults renders MCP code-search results as a compact path list.
func formatSearchResults(results []map[string]interface{}) string {
	if len(results) == 0 {
		return "No matches found."
	}
	var paths []string
	for _, r := range results {
		if p, ok := r["path"].(string); ok && p != "" {
			paths = append(paths, p)
			continue
		}
		if n, ok := r["name"].(string); ok && n != "" {
			paths = append(paths, n)
		}
	}
	if len(paths) == 0 {
		return fmt.Sprintf("%d matches (paths unavailable).", len(results))
	}
	return "Matches:\n- " + strings.Join(paths, "\n- ")
}

// formatChunks renders retrieved RAG chunks as a readable, cited context block.
func formatChunks(chunks []rag.Chunk) string {
	if len(chunks) == 0 {
		return "No relevant indexed code found."
	}
	var sb strings.Builder
	for i, c := range chunks {
		sb.WriteString(fmt.Sprintf("### %s (lines %d-%d)\n```\n%s\n```\n", c.Path, c.StartLine, c.EndLine, c.Content))
		if i < len(chunks)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// truncate caps a string to n characters with an ellipsis marker.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…(truncated)"
}
