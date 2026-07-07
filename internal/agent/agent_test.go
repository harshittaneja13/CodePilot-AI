package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/codepilot-ai/codepilot-ai/internal/llm"
	"github.com/codepilot-ai/codepilot-ai/internal/models"
	"github.com/codepilot-ai/codepilot-ai/internal/rag"
)

// fakeLLM returns a scripted sequence of tool-chat responses, one per call.
type fakeLLM struct {
	responses []*llm.ToolChatResponse
	calls     int
}

func (f *fakeLLM) ChatWithTools(_ context.Context, _ []llm.AgentMessage, _ []llm.ToolDefinition, _ string) (*llm.ToolChatResponse, error) {
	if f.calls >= len(f.responses) {
		return nil, fmt.Errorf("fakeLLM: no scripted response for call %d", f.calls+1)
	}
	r := f.responses[f.calls]
	f.calls++
	return r, nil
}

// fakeMCP serves file contents from a map and returns a canned search result.
type fakeMCP struct {
	files    map[string]string
	getCalls int
}

func (m *fakeMCP) GetFileContents(_ context.Context, _, _, path, _ string) (string, error) {
	m.getCalls++
	if c, ok := m.files[path]; ok {
		return c, nil
	}
	return "", fmt.Errorf("file not found: %s", path)
}

func (m *fakeMCP) SearchCode(_ context.Context, _ string) ([]map[string]interface{}, error) {
	return []map[string]interface{}{{"path": "found.go"}}, nil
}

func newTestInput() Input {
	return Input{
		Owner: "o", Repo: "r", HeadRef: "sha",
		PR:    &models.PullRequest{Title: "Test PR"},
		Files: []llm.FileContext{{Path: "main.go", Patch: "@@ -1 +1 @@\n-old\n+new", Language: "go"}},
	}
}

func toolCallResp(id, name, args string, tokens int) *llm.ToolChatResponse {
	return &llm.ToolChatResponse{
		ToolCalls:  []llm.ToolCall{{ID: id, Name: name, Arguments: args}},
		TokensUsed: tokens,
	}
}

func TestAgentSubmitsFindings(t *testing.T) {
	submitArgs := `{"summary":"looks ok","findings":[{"file_path":"main.go","line_number":1,"title":"t","explanation":"e","severity":"high","confidence":0.9}]}`
	f := &fakeLLM{responses: []*llm.ToolChatResponse{
		toolCallResp("1", toolGetFileDiff, `{"path":"main.go"}`, 10),
		toolCallResp("2", toolSubmitFindings, submitArgs, 20),
	}}

	ag := New(f, &fakeMCP{}, zerolog.Nop())
	res, err := ag.Run(context.Background(), newTestInput())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Summary != "looks ok" {
		t.Errorf("summary = %q, want %q", res.Summary, "looks ok")
	}
	if len(res.Findings) != 1 || res.Findings[0].Severity != "high" {
		t.Fatalf("findings = %+v", res.Findings)
	}
	if res.TokensUsed != 30 {
		t.Errorf("tokens = %d, want 30", res.TokensUsed)
	}
	if res.Steps != 2 {
		t.Errorf("steps = %d, want 2", res.Steps)
	}
	if len(res.Trace) != 2 {
		t.Errorf("trace len = %d, want 2", len(res.Trace))
	}
}

func TestAgentExceedsMaxSteps(t *testing.T) {
	f := &fakeLLM{responses: []*llm.ToolChatResponse{
		toolCallResp("1", toolGetFileDiff, `{"path":"main.go"}`, 5),
		toolCallResp("2", toolGetFileContents, `{"path":"main.go"}`, 5),
	}}
	m := &fakeMCP{files: map[string]string{"main.go": "package main"}}

	ag := New(f, m, zerolog.Nop())
	ag.SetMaxSteps(2)
	_, err := ag.Run(context.Background(), newTestInput())
	if err == nil {
		t.Fatal("expected error when max steps exceeded")
	}
	if !strings.Contains(err.Error(), "max steps") {
		t.Errorf("error = %v, want it to mention max steps", err)
	}
}

func TestAgentErrorsWhenNoToolCalls(t *testing.T) {
	// A model that answers in plain prose (no tools, unparseable) should error so the
	// engine falls back to the deterministic pipeline.
	f := &fakeLLM{responses: []*llm.ToolChatResponse{
		{Content: "I'm not sure how to review this.", TokensUsed: 3},
	}}
	ag := New(f, &fakeMCP{}, zerolog.Nop())
	if _, err := ag.Run(context.Background(), newTestInput()); err == nil {
		t.Fatal("expected error signalling fallback")
	}
}

func TestAgentDedupsRepeatedToolCalls(t *testing.T) {
	f := &fakeLLM{responses: []*llm.ToolChatResponse{
		toolCallResp("1", toolGetFileDiff, `{"path":"main.go"}`, 5),
		toolCallResp("2", toolGetFileDiff, `{"path":"main.go"}`, 5),
		toolCallResp("3", toolSubmitFindings, `{"summary":"s","findings":[]}`, 5),
	}}
	ag := New(f, &fakeMCP{}, zerolog.Nop())
	res, err := ag.Run(context.Background(), newTestInput())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Trace) != 3 {
		t.Fatalf("trace len = %d, want 3", len(res.Trace))
	}
	if !strings.Contains(res.Trace[1].Result, "already called") {
		t.Errorf("second trace result = %q, want dedup note", res.Trace[1].Result)
	}
	if len(res.Findings) != 0 {
		t.Errorf("findings = %d, want 0", len(res.Findings))
	}
}

func TestAgentRecoversFromBadSubmit(t *testing.T) {
	f := &fakeLLM{responses: []*llm.ToolChatResponse{
		toolCallResp("1", toolSubmitFindings, `{invalid json`, 5),
		toolCallResp("2", toolSubmitFindings, `{"summary":"ok","findings":[]}`, 5),
	}}
	ag := New(f, &fakeMCP{}, zerolog.Nop())
	res, err := ag.Run(context.Background(), newTestInput())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Summary != "ok" {
		t.Errorf("summary = %q, want ok", res.Summary)
	}
}

// fakeRetriever returns a canned chunk and counts calls.
type fakeRetriever struct{ calls int }

func (f *fakeRetriever) Retrieve(_ context.Context, _, _ string, _ int) ([]rag.Chunk, error) {
	f.calls++
	return []rag.Chunk{{Path: "lib.go", StartLine: 1, EndLine: 5, Content: "func Foo() {}"}}, nil
}

func TestAgentUsesRetrieveContext(t *testing.T) {
	f := &fakeLLM{responses: []*llm.ToolChatResponse{
		toolCallResp("1", toolRetrieveContext, `{"query":"where is Foo defined"}`, 5),
		toolCallResp("2", toolSubmitFindings, `{"summary":"ok","findings":[]}`, 5),
	}}
	r := &fakeRetriever{}
	ag := New(f, &fakeMCP{}, zerolog.Nop())
	ag.SetRetriever(r)

	res, err := ag.Run(context.Background(), newTestInput())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.calls != 1 {
		t.Errorf("retriever calls = %d, want 1", r.calls)
	}
	if len(res.Trace) < 1 || !strings.Contains(res.Trace[0].Result, "lib.go") {
		t.Errorf("expected retrieved chunk in trace, got %+v", res.Trace)
	}
}

func TestAgentUsesFileContentsTool(t *testing.T) {
	f := &fakeLLM{responses: []*llm.ToolChatResponse{
		toolCallResp("1", toolGetFileContents, `{"path":"util.go"}`, 5),
		toolCallResp("2", toolSubmitFindings, `{"summary":"done","findings":[]}`, 5),
	}}
	m := &fakeMCP{files: map[string]string{"util.go": "package util\nfunc Helper() {}"}}
	ag := New(f, m, zerolog.Nop())
	if _, err := ag.Run(context.Background(), newTestInput()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.getCalls != 1 {
		t.Errorf("GetFileContents calls = %d, want 1", m.getCalls)
	}
}
