package agent

import "github.com/codepilot-ai/codepilot-ai/internal/llm"

// Tool names the agent exposes to the model.
const (
	toolGetFileContents = "get_file_contents"
	toolSearchCode      = "search_code"
	toolGetFileDiff     = "get_file_diff"
	toolRetrieveContext = "retrieve_context"
	toolSubmitFindings  = "submit_findings"
)

// toolDefinitions returns the JSON-schema tool definitions offered to the model.
// get_file_contents / search_code reach the repo via MCP; get_file_diff reads the
// already-fetched PR patches; retrieve_context (only when a RAG retriever is present)
// does semantic search over indexed repo code; submit_findings ends the loop.
func toolDefinitions(hasRetriever bool) []llm.ToolDefinition {
	str := map[string]interface{}{"type": "string"}
	defs := []llm.ToolDefinition{
		{
			Name:        toolGetFileContents,
			Description: "Read the full current contents of a file in the repository at the PR's head commit. Use this to inspect code surrounding the diff, definitions, or callers before judging a change.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string", "description": "Repository-relative file path"},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        toolSearchCode,
			Description: "Search the repository for code matching a query (symbol, function name, or string). Returns matching file paths. Use to find where something is defined or used.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string", "description": "Search query"},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        toolGetFileDiff,
			Description: "Get the diff/patch for one of the files changed in this PR.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string", "description": "Path of a changed file"},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        toolSubmitFindings,
			Description: "Submit the completed code review. Call this exactly once when your analysis is done, even if there are no issues (use an empty findings array).",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"summary": map[string]interface{}{"type": "string", "description": "2-3 sentence overview of PR quality and key concerns"},
					"findings": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"file_path":      str,
								"line_number":    map[string]interface{}{"type": "integer", "description": "A line number present in the diff, or 0"},
								"title":          str,
								"explanation":    str,
								"why_it_matters": str,
								"suggestion":     str,
								"severity":       map[string]interface{}{"type": "string", "enum": []string{"critical", "high", "medium", "low"}},
								"code_snippet":   str,
								"confidence":     map[string]interface{}{"type": "number", "description": "0.0-1.0 honesty about whether this is a genuine issue"},
							},
							"required": []string{"file_path", "title", "explanation", "severity"},
						},
					},
				},
				"required": []string{"summary", "findings"},
			},
		},
	}

	if hasRetriever {
		defs = append(defs, llm.ToolDefinition{
			Name:        toolRetrieveContext,
			Description: "Semantic search over the indexed repository code. Returns the code snippets most relevant to your query (e.g. a function's definition, related implementations). Use this to find context beyond the files changed in this PR.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string", "description": "Natural-language or code query describing what you need to understand"},
				},
				"required": []string{"query"},
			},
		})
	}

	return defs
}
