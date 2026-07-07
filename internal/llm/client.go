package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// rateLimitError is returned when the provider responds with 429 Too Many Requests.
// It carries the suggested retry delay parsed from the error body so the caller
// can wait exactly as long as needed instead of guessing with fixed backoff.
type rateLimitError struct {
	Message string
	WaitFor time.Duration
}

func (e *rateLimitError) Error() string { return e.Message }

// jsonModeError is returned when the provider rejects the request because it
// could not generate valid JSON (Groq error code "json_validate_failed").
// The caller should retry without response_format and fall back to text parsing.
type jsonModeError struct {
	Message string
}

func (e *jsonModeError) Error() string { return e.Message }

// fatalError wraps a non-retriable error (e.g. HTTP 400 bad request).
// The retry loop breaks immediately on this type without waiting or re-sending.
type fatalError struct{ err error }

func (e *fatalError) Error() string { return e.err.Error() }
func (e *fatalError) Unwrap() error { return e.err }

// tryAgainRegexp matches "Please try again in 1.88s" in Groq/OpenAI 429 bodies.
var tryAgainRegexp = regexp.MustCompile(`try again in ([0-9]+(?:\.[0-9]+)?)s`)

// parseTryAgainDuration extracts the suggested wait duration from a rate-limit error body.
// Returns 0 if the pattern is not found.
func parseTryAgainDuration(body string) time.Duration {
	m := tryAgainRegexp.FindStringSubmatch(body)
	if len(m) < 2 {
		return 0
	}
	secs, err := strconv.ParseFloat(m[1], 64)
	if err != nil || secs <= 0 {
		return 0
	}
	return time.Duration(secs*1000) * time.Millisecond
}

// providerBaseURLs maps provider names to their API base URLs.
var providerBaseURLs = map[string]string{
	"openai":    "https://api.openai.com/v1",
	"anthropic": "https://api.anthropic.com/v1",
}

// Client communicates with an OpenAI-compatible LLM API.
type Client struct {
	httpClient  *http.Client
	apiKey      string
	baseURL     string
	provider    string
	model       string
	maxTokens   int
	temperature float64
	jsonMode    bool // when true, requests JSON output via response_format (OpenAI/Groq only)
	disableTools bool // when true, SupportsTools reports false and the engine uses the fixed pipeline
}

// NewClient creates a new LLM Client for the given provider.
func NewClient(provider, apiKey, model string, maxTokens int, temperature float64) *Client {
	return NewClientWithBaseURL(provider, apiKey, "", model, maxTokens, temperature)
}

// NewClientWithBaseURL creates a client and optionally overrides the provider endpoint.
// The compatible provider uses the OpenAI chat-completions wire format.
func NewClientWithBaseURL(provider, apiKey, baseURL, model string, maxTokens int, temperature float64) *Client {
	defaultURL, ok := providerBaseURLs[provider]
	if baseURL != "" {
		baseURL = strings.TrimRight(baseURL, "/")
	} else if ok {
		baseURL = defaultURL
	} else {
		baseURL = providerBaseURLs["openai"]
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		apiKey:      apiKey,
		baseURL:     baseURL,
		provider:    provider,
		model:       model,
		maxTokens:   maxTokens,
		temperature: temperature,
	}
}

type anthropicRequest struct {
	Model       string        `json:"model"`
	System      string        `json:"system,omitempty"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model string `json:"model"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// responseFormat instructs the model to emit valid JSON (OpenAI / Groq JSON mode).
// The system prompt must contain the word "json" for Groq to honour this field.
type responseFormat struct {
	Type string `json:"type"` // "json_object"
}

// openAIRequest matches the OpenAI chat completions request format.
type openAIRequest struct {
	Model          string          `json:"model"`
	Messages       []ChatMessage   `json:"messages"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	Temperature    float64         `json:"temperature"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

// openAIResponse matches the OpenAI chat completions response format.
type openAIResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
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

// ChatJSON sends a chat completion request and enables JSON mode (response_format: json_object).
// Use this for all calls where the LLM is expected to return structured JSON.
func (c *Client) ChatJSON(ctx context.Context, messages []ChatMessage) (*ChatResponse, error) {
	c2 := *c
	c2.jsonMode = true
	return c2.Chat(ctx, messages)
}

// ChatWithModelJSON combines ChatWithModel and JSON mode for structured-output calls.
func (c *Client) ChatWithModelJSON(ctx context.Context, messages []ChatMessage, model string) (*ChatResponse, error) {
	c2 := *c
	c2.jsonMode = true
	if model != "" {
		c2.model = model
	}
	return c2.Chat(ctx, messages)
}

// Chat sends a chat completion request to the LLM provider.
// It retries up to 3 times with exponential backoff on 429 (rate-limited) and 5xx errors.
func (c *Client) Chat(ctx context.Context, messages []ChatMessage) (*ChatResponse, error) {
	var reqBody interface{}
	if c.provider == "anthropic" {
		system, filtered := splitSystemMessage(messages)
		reqBody = anthropicRequest{Model: c.model, System: system, Messages: filtered, MaxTokens: c.maxTokens, Temperature: c.temperature}
	} else {
		req := openAIRequest{Model: c.model, Messages: messages, MaxTokens: c.maxTokens, Temperature: c.temperature}
		if c.jsonMode {
			req.ResponseFormat = &responseFormat{Type: "json_object"}
		}
		reqBody = req
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling chat request: %w", err)
	}

	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			var backoff time.Duration
			var rle *rateLimitError
			if errors.As(lastErr, &rle) && rle.WaitFor > 0 {
				// Wait exactly as long as the provider says, plus a 3-second buffer.
				// This is far more reliable than fixed exponential backoff for TPM limits,
				// where the window may need nearly a full minute to drain.
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

		resp, err := c.doRequest(ctx, bodyBytes)
		if err != nil {
			lastErr = err
			// fatalError (non-retriable 400) and jsonModeError both require a
			// different request, not the same one retried — break immediately.
			var fe *fatalError
			var jme *jsonModeError
			if errors.As(err, &fe) || errors.As(err, &jme) {
				break
			}
			continue
		}

		return resp, nil
	}

	// If JSON mode was the cause, retry once without response_format so that the
	// existing extractJSON / ParseXxx helpers can recover usable output from text.
	var jme *jsonModeError
	if errors.As(lastErr, &jme) && c.jsonMode {
		c2 := *c
		c2.jsonMode = false
		return c2.Chat(ctx, messages)
	}

	return nil, fmt.Errorf("chat request failed after %d retries: %w", maxRetries, lastErr)
}

// sendJSON performs a single HTTP POST to the provider's chat/messages endpoint and
// returns the raw response body. Status codes are mapped to the same typed errors used
// by doRequest (rateLimitError for 429, fatalError for 400, plain errors for 5xx/other)
// so callers can share the retry policy in sendWithRetry.
func (c *Client) sendJSON(ctx context.Context, bodyBytes []byte) ([]byte, error) {
	url := c.baseURL + "/chat/completions"
	if c.provider == "anthropic" {
		url = c.baseURL + "/messages"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.provider == "anthropic" {
		req.Header.Set("x-api-key", c.apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, &rateLimitError{
			Message: fmt.Sprintf("rate limited (status 429): %s", string(respBody)),
			WaitFor: parseTryAgainDuration(string(respBody)),
		}
	case resp.StatusCode >= 500:
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(respBody))
	case resp.StatusCode == http.StatusBadRequest:
		return nil, &fatalError{err: fmt.Errorf("bad request: %s", string(respBody))}
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

// doRequest performs a single HTTP request to the chat completions endpoint.
func (c *Client) doRequest(ctx context.Context, bodyBytes []byte) (*ChatResponse, error) {
	url := c.baseURL + "/chat/completions"
	if c.provider == "anthropic" {
		url = c.baseURL + "/messages"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.provider == "anthropic" {
		req.Header.Set("x-api-key", c.apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	// On 429, parse the suggested wait time so the retry loop can sleep exactly long enough.
	if resp.StatusCode == http.StatusTooManyRequests {
		suggested := parseTryAgainDuration(string(respBody))
		return nil, &rateLimitError{
			Message: fmt.Sprintf("rate limited (status 429): %s", string(respBody)),
			WaitFor: suggested,
		}
	}

	// Retry on server errors
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// 400 Bad Request — most are deterministic and should not be retried.
	// Exception: json_validate_failed means JSON mode caused the failure; the caller
	// can recover by retrying without response_format.
	if resp.StatusCode == http.StatusBadRequest {
		if strings.Contains(string(respBody), "json_validate_failed") {
			return nil, &jsonModeError{Message: fmt.Sprintf("json_validate_failed: %s", string(respBody))}
		}
		return nil, &fatalError{err: fmt.Errorf("bad request: %s", string(respBody))}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	if c.provider == "anthropic" {
		var anthropicResp anthropicResponse
		if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
			return nil, fmt.Errorf("parsing Anthropic response: %w", err)
		}
		if anthropicResp.Error != nil {
			return nil, fmt.Errorf("API error: %s (%s)", anthropicResp.Error.Message, anthropicResp.Error.Type)
		}
		var content strings.Builder
		for _, block := range anthropicResp.Content {
			if block.Type == "text" {
				content.WriteString(block.Text)
			}
		}
		if content.Len() == 0 {
			return nil, fmt.Errorf("no text content in Anthropic response")
		}
		return &ChatResponse{
			Content:      content.String(),
			TokensUsed:   anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
			InputTokens:  anthropicResp.Usage.InputTokens,
			OutputTokens: anthropicResp.Usage.OutputTokens,
			Model:        anthropicResp.Model,
		}, nil
	}

	var openAIResp openAIResponse
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if openAIResp.Error != nil {
		return nil, fmt.Errorf("API error: %s (%s)", openAIResp.Error.Message, openAIResp.Error.Type)
	}

	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &ChatResponse{
		Content:      openAIResp.Choices[0].Message.Content,
		TokensUsed:   openAIResp.Usage.TotalTokens,
		InputTokens:  openAIResp.Usage.PromptTokens,
		OutputTokens: openAIResp.Usage.CompletionTokens,
		Model:        openAIResp.Model,
	}, nil
}

func splitSystemMessage(messages []ChatMessage) (string, []ChatMessage) {
	var systemParts []string
	filtered := make([]ChatMessage, 0, len(messages))
	for _, message := range messages {
		if message.Role == "system" {
			systemParts = append(systemParts, message.Content)
			continue
		}
		filtered = append(filtered, message)
	}
	return strings.Join(systemParts, "\n\n"), filtered
}

// GetModel returns the configured model name.
func (c *Client) GetModel() string {
	return c.model
}

// ChatWithModel is like Chat but overrides the model for this single call.
func (c *Client) ChatWithModel(ctx context.Context, messages []ChatMessage, model string) (*ChatResponse, error) {
	if model == "" {
		return c.Chat(ctx, messages)
	}
	override := *c
	override.model = model
	return override.Chat(ctx, messages)
}

// ListModels fetches available models from the provider endpoint.
// Returns an empty slice if the provider does not support listing.
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	if c.provider == "anthropic" {
		req.Header.Set("x-api-key", c.apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		if m.ID != "" {
			models = append(models, m.ID)
		}
	}
	return models, nil
}
