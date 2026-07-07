package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// ToolDefinition describes a tool the model may call during a tool-use loop.
// Parameters is a JSON Schema object describing the tool's arguments.
type ToolDefinition struct {
	Name        string
	Description string
	Parameters  map[string]interface{}
}

// ToolCall is a single tool invocation requested by the model.
type ToolCall struct {
	ID        string
	Name      string
	Arguments string // raw JSON string of the arguments object
}

// AgentMessage is a provider-agnostic conversation message that supports tool use.
// Role is one of: "system", "user", "assistant", "tool".
//   - assistant messages that request tools set ToolCalls.
//   - tool-result messages (Role=="tool") set ToolCallID to the call they answer.
type AgentMessage struct {
	Role       string
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
}

// ToolChatResponse is the result of a single tool-enabled chat turn. Either
// ToolCalls is non-empty (the model wants to call tools) or Content holds the
// model's final text.
type ToolChatResponse struct {
	Content      string
	ToolCalls    []ToolCall
	TokensUsed   int // total (input + output)
	InputTokens  int
	OutputTokens int
	Model        string
}

// SupportsTools reports whether tool-calling should be attempted for this client.
// The engine uses this to decide between the agent loop and the fixed pipeline.
func (c *Client) SupportsTools() bool { return !c.disableTools }

// SetToolSupport enables or disables tool-calling for this client.
func (c *Client) SetToolSupport(enabled bool) { c.disableTools = !enabled }

// ChatWithTools performs one tool-enabled chat turn, returning either the model's
// requested tool calls or its final text. Works with OpenAI-compatible providers
// (OpenAI, Groq) and Anthropic. If model is non-empty it overrides the client default.
func (c *Client) ChatWithTools(ctx context.Context, messages []AgentMessage, tools []ToolDefinition, model string) (*ToolChatResponse, error) {
	m := c.model
	if model != "" {
		m = model
	}
	if c.provider == "anthropic" {
		return c.chatWithToolsAnthropic(ctx, m, messages, tools)
	}
	return c.chatWithToolsOpenAI(ctx, m, messages, tools)
}

// sendWithRetry posts a request body to the provider endpoint, retrying on 429 and
// 5xx with the same backoff policy as Chat. Returns the raw response body.
func (c *Client) sendWithRetry(ctx context.Context, bodyBytes []byte) ([]byte, error) {
	const maxRetries = 3
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			var backoff time.Duration
			var rle *rateLimitError
			if errors.As(lastErr, &rle) && rle.WaitFor > 0 {
				backoff = rle.WaitFor + 3*time.Second
			} else {
				backoff = time.Duration(math.Pow(2, float64(attempt))) * time.Second
			}
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("context canceled during retry: %w", ctx.Err())
			case <-time.After(backoff):
			}
		}
		body, err := c.sendJSON(ctx, bodyBytes)
		if err != nil {
			lastErr = err
			var fe *fatalError
			if errors.As(err, &fe) {
				break
			}
			continue
		}
		return body, nil
	}
	return nil, fmt.Errorf("tool request failed after %d retries: %w", maxRetries, lastErr)
}

// ── OpenAI-compatible tool calling ───────────────────────────────────────────

type openAITool struct {
	Type     string             `json:"type"`
	Function openAIToolFunction `json:"function"`
}

type openAIToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type openAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openAIToolMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openAIToolRequest struct {
	Model       string              `json:"model"`
	Messages    []openAIToolMessage `json:"messages"`
	Tools       []openAITool        `json:"tools,omitempty"`
	ToolChoice  string              `json:"tool_choice,omitempty"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Temperature float64             `json:"temperature"`
}

type openAIToolResponse struct {
	Choices []struct {
		Message struct {
			Content   string           `json:"content"`
			ToolCalls []openAIToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Model string `json:"model"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func (c *Client) chatWithToolsOpenAI(ctx context.Context, model string, messages []AgentMessage, tools []ToolDefinition) (*ToolChatResponse, error) {
	reqTools := make([]openAITool, 0, len(tools))
	for _, t := range tools {
		reqTools = append(reqTools, openAITool{
			Type:     "function",
			Function: openAIToolFunction(t),
		})
	}

	msgs := make([]openAIToolMessage, 0, len(messages))
	for _, m := range messages {
		om := openAIToolMessage{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID}
		for _, tc := range m.ToolCalls {
			oc := openAIToolCall{ID: tc.ID, Type: "function"}
			oc.Function.Name = tc.Name
			oc.Function.Arguments = tc.Arguments
			om.ToolCalls = append(om.ToolCalls, oc)
		}
		msgs = append(msgs, om)
	}

	reqBody := openAIToolRequest{Model: model, Messages: msgs, MaxTokens: c.maxTokens, Temperature: c.temperature}
	if len(reqTools) > 0 {
		reqBody.Tools = reqTools
		reqBody.ToolChoice = "auto"
	}

	b, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling tool request: %w", err)
	}
	respBody, err := c.sendWithRetry(ctx, b)
	if err != nil {
		return nil, err
	}

	var r openAIToolResponse
	if err := json.Unmarshal(respBody, &r); err != nil {
		return nil, fmt.Errorf("parsing tool response: %w", err)
	}
	if r.Error != nil {
		return nil, fmt.Errorf("API error: %s (%s)", r.Error.Message, r.Error.Type)
	}
	if len(r.Choices) == 0 {
		return nil, fmt.Errorf("no choices in tool response")
	}

	msg := r.Choices[0].Message
	out := &ToolChatResponse{
		Content:      msg.Content,
		TokensUsed:   r.Usage.TotalTokens,
		InputTokens:  r.Usage.PromptTokens,
		OutputTokens: r.Usage.CompletionTokens,
		Model:        r.Model,
	}
	for _, tc := range msg.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, ToolCall{ID: tc.ID, Name: tc.Function.Name, Arguments: tc.Function.Arguments})
	}
	return out, nil
}

// ── Anthropic tool calling ───────────────────────────────────────────────────

type anthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type anthropicContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
}

type anthropicToolMessage struct {
	Role    string                  `json:"role"`
	Content []anthropicContentBlock `json:"content"`
}

type anthropicToolRequest struct {
	Model       string                 `json:"model"`
	System      string                 `json:"system,omitempty"`
	Messages    []anthropicToolMessage `json:"messages"`
	Tools       []anthropicTool        `json:"tools,omitempty"`
	MaxTokens   int                    `json:"max_tokens"`
	Temperature float64                `json:"temperature"`
}

type anthropicToolResponse struct {
	Content    []anthropicContentBlock `json:"content"`
	Model      string                  `json:"model"`
	StopReason string                  `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func (c *Client) chatWithToolsAnthropic(ctx context.Context, model string, messages []AgentMessage, tools []ToolDefinition) (*ToolChatResponse, error) {
	reqTools := make([]anthropicTool, 0, len(tools))
	for _, t := range tools {
		reqTools = append(reqTools, anthropicTool{Name: t.Name, Description: t.Description, InputSchema: t.Parameters})
	}

	var system string
	var amsgs []anthropicToolMessage
	for _, m := range messages {
		switch m.Role {
		case "system":
			if system != "" {
				system += "\n\n"
			}
			system += m.Content
		case "tool":
			// Anthropic requires all tool_result blocks answering one assistant turn
			// to live in a single user message; merge consecutive tool results.
			block := anthropicContentBlock{Type: "tool_result", ToolUseID: m.ToolCallID, Content: m.Content}
			if n := len(amsgs); n > 0 && amsgs[n-1].Role == "user" &&
				len(amsgs[n-1].Content) > 0 && amsgs[n-1].Content[0].Type == "tool_result" {
				amsgs[n-1].Content = append(amsgs[n-1].Content, block)
			} else {
				amsgs = append(amsgs, anthropicToolMessage{Role: "user", Content: []anthropicContentBlock{block}})
			}
		case "assistant":
			var blocks []anthropicContentBlock
			if m.Content != "" {
				blocks = append(blocks, anthropicContentBlock{Type: "text", Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				input := json.RawMessage(tc.Arguments)
				if len(input) == 0 {
					input = json.RawMessage("{}")
				}
				blocks = append(blocks, anthropicContentBlock{Type: "tool_use", ID: tc.ID, Name: tc.Name, Input: input})
			}
			amsgs = append(amsgs, anthropicToolMessage{Role: "assistant", Content: blocks})
		default: // user
			amsgs = append(amsgs, anthropicToolMessage{Role: "user", Content: []anthropicContentBlock{{Type: "text", Text: m.Content}}})
		}
	}

	reqBody := anthropicToolRequest{Model: model, System: system, Messages: amsgs, MaxTokens: c.maxTokens, Temperature: c.temperature}
	if len(reqTools) > 0 {
		reqBody.Tools = reqTools
	}

	b, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling tool request: %w", err)
	}
	respBody, err := c.sendWithRetry(ctx, b)
	if err != nil {
		return nil, err
	}

	var r anthropicToolResponse
	if err := json.Unmarshal(respBody, &r); err != nil {
		return nil, fmt.Errorf("parsing tool response: %w", err)
	}
	if r.Error != nil {
		return nil, fmt.Errorf("API error: %s (%s)", r.Error.Message, r.Error.Type)
	}

	out := &ToolChatResponse{
		TokensUsed:   r.Usage.InputTokens + r.Usage.OutputTokens,
		InputTokens:  r.Usage.InputTokens,
		OutputTokens: r.Usage.OutputTokens,
		Model:        r.Model,
	}
	var text strings.Builder
	for _, blk := range r.Content {
		switch blk.Type {
		case "text":
			text.WriteString(blk.Text)
		case "tool_use":
			out.ToolCalls = append(out.ToolCalls, ToolCall{ID: blk.ID, Name: blk.Name, Arguments: string(blk.Input)})
		}
	}
	out.Content = text.String()
	return out, nil
}
