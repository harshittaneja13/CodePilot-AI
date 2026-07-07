package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/codepilot-ai/codepilot-ai/internal/mcp"
	"github.com/codepilot-ai/codepilot-ai/internal/models"
	"github.com/codepilot-ai/codepilot-ai/internal/rag"
)

type fakeReviews struct{}

func (fakeReviews) GetByID(_ context.Context, id string) (*models.ReviewWithComments, error) {
	if id == "missing" {
		return nil, fmt.Errorf("review not found")
	}
	line := 42
	return &models.ReviewWithComments{
		Review: models.Review{
			ID: id, Status: "completed", LLMModelStr: "llama-3.3-70b",
			TotalComments: 1, HighCount: 1, TokensUsed: 100, InputTokens: 80, OutputTokens: 20,
			CostUSD: 0.0123, SummaryStr: "looks good overall",
		},
		Comments: []models.ReviewComment{{
			FilePath: "main.go", LineNumberVal: &line, Severity: "high",
			Title: "possible nil deref", Explanation: "x may be nil", SuggestionStr: "add a check",
		}},
	}, nil
}

func (fakeReviews) ListRecent(_ context.Context, _ int) ([]models.Review, error) {
	return []models.Review{{ID: "r1", Status: "completed", LLMModelStr: "m", TotalComments: 2, CostUSD: 0.01}}, nil
}

type fakeRetriever struct{}

func (fakeRetriever) Retrieve(_ context.Context, _, _ string, _ int) ([]rag.Chunk, error) {
	return []rag.Chunk{{Path: "lib.go", StartLine: 1, EndLine: 3, Content: "func X() {}"}}, nil
}

func runServer(t *testing.T, retriever ContextRetriever, requests ...string) []mcp.Response {
	t.Helper()
	s := New("codepilot-ai", "1.0.0", zerolog.Nop())
	RegisterCodePilotTools(s, fakeReviews{}, retriever)

	in := strings.NewReader(strings.Join(requests, "\n") + "\n")
	var out bytes.Buffer
	if err := s.Serve(context.Background(), in, &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	var resps []mcp.Response
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		var r mcp.Response
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("unmarshal response %q: %v", line, err)
		}
		resps = append(resps, r)
	}
	return resps
}

func callResult(t *testing.T, r mcp.Response) mcp.CallToolResult {
	t.Helper()
	var res mcp.CallToolResult
	if err := json.Unmarshal(r.Result, &res); err != nil {
		t.Fatalf("unmarshal CallToolResult: %v", err)
	}
	return res
}

func TestInitialize(t *testing.T) {
	resps := runServer(t, nil, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	if len(resps) != 1 {
		t.Fatalf("got %d responses, want 1", len(resps))
	}
	var init mcp.InitializeResult
	if err := json.Unmarshal(resps[0].Result, &init); err != nil {
		t.Fatalf("unmarshal init result: %v", err)
	}
	if init.ProtocolVersion != protocolVersion {
		t.Errorf("protocolVersion = %q, want %q", init.ProtocolVersion, protocolVersion)
	}
	if init.ServerInfo.Name != "codepilot-ai" {
		t.Errorf("serverInfo.name = %q", init.ServerInfo.Name)
	}
}

func TestToolsListRespectsRetriever(t *testing.T) {
	names := func(retriever ContextRetriever) map[string]bool {
		resps := runServer(t, retriever, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
		var list mcp.ListToolsResult
		if err := json.Unmarshal(resps[0].Result, &list); err != nil {
			t.Fatalf("unmarshal tools/list: %v", err)
		}
		set := map[string]bool{}
		for _, tool := range list.Tools {
			set[tool.Name] = true
		}
		return set
	}

	with := names(fakeRetriever{})
	for _, want := range []string{"get_review", "list_findings", "search_reviews", "retrieve_code_context"} {
		if !with[want] {
			t.Errorf("with retriever: missing tool %q", want)
		}
	}
	without := names(nil)
	if without["retrieve_code_context"] {
		t.Error("without retriever: retrieve_code_context should not be registered")
	}
	if !without["get_review"] {
		t.Error("without retriever: get_review should still be registered")
	}
}

func TestCallGetReview(t *testing.T) {
	resps := runServer(t, nil,
		`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"get_review","arguments":{"review_id":"abc"}}}`)
	res := callResult(t, resps[0])
	if res.IsError {
		t.Fatalf("unexpected isError; content=%v", res.Content)
	}
	text := res.Content[0].Text
	for _, want := range []string{"looks good overall", "possible nil deref", "main.go:42", "$0.0123"} {
		if !strings.Contains(text, want) {
			t.Errorf("get_review text missing %q; got:\n%s", want, text)
		}
	}
}

func TestCallGetReviewError(t *testing.T) {
	resps := runServer(t, nil,
		`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"get_review","arguments":{"review_id":"missing"}}}`)
	res := callResult(t, resps[0])
	if !res.IsError {
		t.Fatal("expected isError for missing review")
	}
	if !strings.Contains(res.Content[0].Text, "review not found") {
		t.Errorf("error text = %q", res.Content[0].Text)
	}
}

func TestRetrieveCodeContext(t *testing.T) {
	resps := runServer(t, fakeRetriever{},
		`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"retrieve_code_context","arguments":{"repo":"o/r","query":"where is X"}}}`)
	res := callResult(t, resps[0])
	if res.IsError || !strings.Contains(res.Content[0].Text, "lib.go") {
		t.Errorf("retrieve_code_context result = %+v", res)
	}
}

func TestNotificationsGetNoResponse(t *testing.T) {
	// initialize (id) → response; notifications/initialized (no id) → no response; ping (id) → response.
	resps := runServer(t, nil,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`)
	if len(resps) != 2 {
		t.Fatalf("got %d responses, want 2 (notification must not reply)", len(resps))
	}
}

func TestUnknownMethod(t *testing.T) {
	resps := runServer(t, nil, `{"jsonrpc":"2.0","id":5,"method":"does/not/exist"}`)
	if resps[0].Error == nil || resps[0].Error.Code != -32601 {
		t.Errorf("expected method-not-found error, got %+v", resps[0])
	}
}
