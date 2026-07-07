package eval

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog"

	"github.com/codepilot-ai/codepilot-ai/internal/agent"
	"github.com/codepilot-ai/codepilot-ai/internal/llm"
	"github.com/codepilot-ai/codepilot-ai/internal/models"
)

// AgentReviewer runs the real review agent over a case, serving tool calls from the
// case's local fixture files (no GitHub/MCP). It requires a working LLM client, so it
// is used by cmd/eval (with an API key), not by the package's unit tests.
type AgentReviewer struct {
	client *llm.Client
}

// NewAgentReviewer builds a reviewer backed by the given LLM client.
func NewAgentReviewer(client *llm.Client) *AgentReviewer { return &AgentReviewer{client: client} }

// Review runs the agent and returns its findings for the case.
func (a *AgentReviewer) Review(ctx context.Context, c Case) ([]llm.ReviewFinding, error) {
	files := make([]llm.FileContext, 0, len(c.PR.Files))
	contents := make(map[string]string, len(c.PR.Files))
	for _, f := range c.PR.Files {
		files = append(files, llm.FileContext{Path: f.Path, Patch: f.Patch, Language: f.Language})
		if f.Content != "" {
			contents[f.Path] = f.Content
		}
	}

	ag := agent.New(a.client, &fixtureTools{files: contents}, zerolog.Nop())
	res, err := ag.Run(ctx, agent.Input{
		Owner: "eval",
		Repo:  c.Name,
		PR:    &models.PullRequest{Title: c.PR.Title, BodyStr: c.PR.Body},
		Files: files,
		Model: a.client.GetModel(),
	})
	if err != nil {
		return nil, err
	}
	return res.Findings, nil
}

// fixtureTools implements agent.MCPTools over a case's local files.
type fixtureTools struct{ files map[string]string }

func (t *fixtureTools) GetFileContents(_ context.Context, _, _, path, _ string) (string, error) {
	if c, ok := t.files[path]; ok {
		return c, nil
	}
	return "", fmt.Errorf("file not found: %s", path)
}

func (t *fixtureTools) SearchCode(_ context.Context, query string) ([]map[string]interface{}, error) {
	q := strings.ToLower(query)
	var out []map[string]interface{}
	for path, content := range t.files {
		if strings.Contains(strings.ToLower(path+"\n"+content), q) {
			out = append(out, map[string]interface{}{"path": path})
		}
	}
	return out, nil
}
