package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Embedder turns text into vectors. Implementations are pluggable so the review
// engine is agnostic to the embedding provider.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Dim returns the embedding dimensionality (vector length).
	Dim() int
}

// HTTPEmbedder calls an OpenAI-compatible /embeddings endpoint. It works with a
// local Ollama sidecar (base URL http://ollama:11434/v1, model nomic-embed-text)
// as well as hosted OpenAI-compatible providers.
type HTTPEmbedder struct {
	client  *http.Client
	baseURL string
	model   string
	apiKey  string
	dim     int
}

// NewHTTPEmbedder builds an embeddings client. baseURL should NOT include the
// trailing /embeddings path. apiKey may be empty (Ollama needs none).
func NewHTTPEmbedder(baseURL, model, apiKey string, dim int) *HTTPEmbedder {
	return &HTTPEmbedder{
		client:  &http.Client{Timeout: 60 * time.Second},
		baseURL: trimSlash(baseURL),
		model:   model,
		apiKey:  apiKey,
		dim:     dim,
	}
}

func (e *HTTPEmbedder) Dim() int { return e.dim }

type embeddingsRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingsResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Embed returns one vector per input text, in the same order.
func (e *HTTPEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(embeddingsRequest{Model: e.model, Input: texts})
	if err != nil {
		return nil, fmt.Errorf("marshaling embeddings request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating embeddings request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling embeddings endpoint: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embeddings endpoint returned %d: %s", resp.StatusCode, string(raw))
	}

	var out embeddingsResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("parsing embeddings response: %w", err)
	}
	if out.Error != nil {
		return nil, fmt.Errorf("embeddings error: %s", out.Error.Message)
	}
	if len(out.Data) != len(texts) {
		return nil, fmt.Errorf("embeddings count mismatch: got %d for %d inputs", len(out.Data), len(texts))
	}

	vecs := make([][]float32, len(out.Data))
	for i, d := range out.Data {
		vecs[i] = d.Embedding
	}
	return vecs, nil
}
