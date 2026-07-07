package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// VectorStore persists and searches embedded chunks. Abstracted so the service can
// be tested against a fake and so Qdrant could be swapped later.
type VectorStore interface {
	// EnsureCollection creates the collection with the given vector size if absent.
	EnsureCollection(ctx context.Context, dim int) error
	// Upsert inserts or replaces points.
	Upsert(ctx context.Context, points []Point) error
	// Search returns the k most similar chunks for a query vector, scoped to repo.
	Search(ctx context.Context, repo string, vector []float32, k int) ([]Chunk, error)
}

// Point is a vector plus the chunk it was derived from.
type Point struct {
	ID     string
	Vector []float32
	Chunk  Chunk
}

// QdrantStore talks to Qdrant over its REST API.
type QdrantStore struct {
	client     *http.Client
	baseURL    string
	collection string
}

// NewQdrantStore builds a Qdrant client. baseURL is e.g. http://qdrant:6333.
func NewQdrantStore(baseURL, collection string) *QdrantStore {
	return &QdrantStore{
		client:     &http.Client{Timeout: 30 * time.Second},
		baseURL:    trimSlash(baseURL),
		collection: collection,
	}
}

func (q *QdrantStore) EnsureCollection(ctx context.Context, dim int) error {
	// Already exists?
	getURL := fmt.Sprintf("%s/collections/%s", q.baseURL, q.collection)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, getURL, nil)
	resp, err := q.client.Do(req)
	if err != nil {
		return fmt.Errorf("checking Qdrant collection: %w", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	// Create it.
	create := map[string]interface{}{
		"vectors": map[string]interface{}{"size": dim, "distance": "Cosine"},
	}
	if err := q.do(ctx, http.MethodPut, fmt.Sprintf("/collections/%s", q.collection), create, nil); err != nil {
		return fmt.Errorf("creating Qdrant collection: %w", err)
	}
	return nil
}

type qdrantPoint struct {
	ID      string                 `json:"id"`
	Vector  []float32              `json:"vector"`
	Payload map[string]interface{} `json:"payload"`
}

func (q *QdrantStore) Upsert(ctx context.Context, points []Point) error {
	if len(points) == 0 {
		return nil
	}
	qps := make([]qdrantPoint, 0, len(points))
	for _, p := range points {
		qps = append(qps, qdrantPoint{
			ID:     p.ID,
			Vector: p.Vector,
			Payload: map[string]interface{}{
				"repo":       p.Chunk.Repo,
				"path":       p.Chunk.Path,
				"commit":     p.Chunk.Commit,
				"language":   p.Chunk.Language,
				"start_line": p.Chunk.StartLine,
				"end_line":   p.Chunk.EndLine,
				"content":    p.Chunk.Content,
			},
		})
	}
	body := map[string]interface{}{"points": qps}
	path := fmt.Sprintf("/collections/%s/points?wait=true", q.collection)
	if err := q.do(ctx, http.MethodPut, path, body, nil); err != nil {
		return fmt.Errorf("upserting points: %w", err)
	}
	return nil
}

type qdrantSearchResponse struct {
	Result []struct {
		Score   float32                `json:"score"`
		Payload map[string]interface{} `json:"payload"`
	} `json:"result"`
	Status interface{} `json:"status"`
}

func (q *QdrantStore) Search(ctx context.Context, repo string, vector []float32, k int) ([]Chunk, error) {
	body := map[string]interface{}{
		"vector":       vector,
		"limit":        k,
		"with_payload": true,
		"filter": map[string]interface{}{
			"must": []map[string]interface{}{
				{"key": "repo", "match": map[string]interface{}{"value": repo}},
			},
		},
	}
	var out qdrantSearchResponse
	path := fmt.Sprintf("/collections/%s/points/search", q.collection)
	if err := q.do(ctx, http.MethodPost, path, body, &out); err != nil {
		return nil, fmt.Errorf("searching points: %w", err)
	}

	chunks := make([]Chunk, 0, len(out.Result))
	for _, r := range out.Result {
		chunks = append(chunks, Chunk{
			Repo:      repo,
			Path:      asString(r.Payload["path"]),
			Commit:    asString(r.Payload["commit"]),
			Language:  asString(r.Payload["language"]),
			StartLine: asInt(r.Payload["start_line"]),
			EndLine:   asInt(r.Payload["end_line"]),
			Content:   asString(r.Payload["content"]),
			Score:     r.Score,
		})
	}
	return chunks, nil
}

// do performs a JSON request and optionally decodes the response into out.
func (q *QdrantStore) do(ctx context.Context, method, path string, body interface{}, out interface{}) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request: %w", err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, q.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := q.client.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("qdrant returned %d: %s", resp.StatusCode, string(raw))
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}

func trimSlash(s string) string { return strings.TrimRight(s, "/") }

func asString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func asInt(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}
