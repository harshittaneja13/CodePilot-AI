package mcpserver

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/codepilot-ai/codepilot-ai/internal/models"
	"github.com/codepilot-ai/codepilot-ai/internal/rag"
)

// ReviewReader is the read side of the review store the MCP tools expose.
type ReviewReader interface {
	GetByID(ctx context.Context, id string) (*models.ReviewWithComments, error)
	ListRecent(ctx context.Context, limit int) ([]models.Review, error)
}

// ContextRetriever is the RAG retrieval capability (optional).
type ContextRetriever interface {
	Retrieve(ctx context.Context, repo, query string, k int) ([]rag.Chunk, error)
}

// RegisterCodePilotTools registers CodePilot's review-history tools, plus the RAG
// retrieval tool when a retriever is supplied (nil disables it).
func RegisterCodePilotTools(s *Server, reviews ReviewReader, retriever ContextRetriever) {
	strProp := func(desc string) map[string]interface{} {
		return map[string]interface{}{"type": "string", "description": desc}
	}
	reviewIDSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{"review_id": strProp("The review UUID")},
		"required":   []string{"review_id"},
	}

	s.Register("get_review",
		"Fetch a CodePilot review by ID: status, summary, severity counts, token/cost usage, and findings.",
		reviewIDSchema,
		func(ctx context.Context, args map[string]interface{}) (string, error) {
			id, _ := args["review_id"].(string)
			if id == "" {
				return "", fmt.Errorf("review_id is required")
			}
			rwc, err := reviews.GetByID(ctx, id)
			if err != nil {
				return "", err
			}
			return formatReview(rwc), nil
		})

	s.Register("list_findings",
		"List the findings (review comments) of a CodePilot review.",
		reviewIDSchema,
		func(ctx context.Context, args map[string]interface{}) (string, error) {
			id, _ := args["review_id"].(string)
			if id == "" {
				return "", fmt.Errorf("review_id is required")
			}
			rwc, err := reviews.GetByID(ctx, id)
			if err != nil {
				return "", err
			}
			return formatFindings(rwc.Comments), nil
		})

	s.Register("search_reviews",
		"List the most recent CodePilot reviews (id, status, model, findings, cost).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"limit": map[string]interface{}{"type": "integer", "description": "Max reviews to return (default 10)"},
			},
		},
		func(ctx context.Context, args map[string]interface{}) (string, error) {
			list, err := reviews.ListRecent(ctx, intArg(args, "limit", 10))
			if err != nil {
				return "", err
			}
			return formatReviewList(list), nil
		})

	if retriever != nil {
		s.Register("retrieve_code_context",
			"Semantic search over indexed repository code (RAG). Returns the code snippets most relevant to a query.",
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"repo":  strProp("Repository full name, e.g. owner/name"),
					"query": strProp("Natural-language or code query"),
					"k":     map[string]interface{}{"type": "integer", "description": "Number of snippets (default 5)"},
				},
				"required": []string{"repo", "query"},
			},
			func(ctx context.Context, args map[string]interface{}) (string, error) {
				repo, _ := args["repo"].(string)
				query, _ := args["query"].(string)
				if repo == "" || query == "" {
					return "", fmt.Errorf("repo and query are required")
				}
				chunks, err := retriever.Retrieve(ctx, repo, query, intArg(args, "k", 5))
				if err != nil {
					return "", err
				}
				return formatChunks(chunks), nil
			})
	}
}

// intArg reads an integer argument (JSON numbers decode as float64), with a default.
func intArg(args map[string]interface{}, key string, def int) int {
	switch n := args[key].(type) {
	case float64:
		return int(n)
	case int:
		return n
	case string:
		if i, err := strconv.Atoi(n); err == nil {
			return i
		}
	}
	return def
}

func formatReview(rwc *models.ReviewWithComments) string {
	r := rwc.Review
	var sb strings.Builder
	fmt.Fprintf(&sb, "Review %s\nStatus: %s\nModel: %s\n", r.ID, r.Status, r.LLMModelStr)
	fmt.Fprintf(&sb, "Findings: %d (critical %d, high %d, medium %d, low %d)\n",
		r.TotalComments, r.CriticalCount, r.HighCount, r.MediumCount, r.LowCount)
	fmt.Fprintf(&sb, "Tokens: %d (in %d / out %d)  Cost: $%.4f\n", r.TokensUsed, r.InputTokens, r.OutputTokens, r.CostUSD)
	if r.SummaryStr != "" {
		fmt.Fprintf(&sb, "Summary: %s\n", r.SummaryStr)
	}
	sb.WriteString("\n")
	sb.WriteString(formatFindings(rwc.Comments))
	return sb.String()
}

func formatFindings(comments []models.ReviewComment) string {
	if len(comments) == 0 {
		return "No findings."
	}
	var sb strings.Builder
	for i, c := range comments {
		loc := c.FilePath
		if c.LineNumberVal != nil {
			loc = fmt.Sprintf("%s:%d", c.FilePath, *c.LineNumberVal)
		}
		fmt.Fprintf(&sb, "%d. [%s] %s (%s)\n   %s\n", i+1, strings.ToUpper(c.Severity), c.Title, loc, c.Explanation)
		if c.SuggestionStr != "" {
			fmt.Fprintf(&sb, "   Suggestion: %s\n", c.SuggestionStr)
		}
	}
	return sb.String()
}

func formatReviewList(list []models.Review) string {
	if len(list) == 0 {
		return "No reviews found."
	}
	var sb strings.Builder
	for _, r := range list {
		fmt.Fprintf(&sb, "- %s  status=%s  model=%s  findings=%d  cost=$%.4f\n",
			r.ID, r.Status, r.LLMModelStr, r.TotalComments, r.CostUSD)
	}
	return sb.String()
}

func formatChunks(chunks []rag.Chunk) string {
	if len(chunks) == 0 {
		return "No relevant indexed code found."
	}
	var sb strings.Builder
	for _, c := range chunks {
		fmt.Fprintf(&sb, "### %s (lines %d-%d)\n```\n%s\n```\n\n", c.Path, c.StartLine, c.EndLine, c.Content)
	}
	return strings.TrimRight(sb.String(), "\n")
}
