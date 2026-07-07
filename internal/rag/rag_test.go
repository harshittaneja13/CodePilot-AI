package rag

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func makeLines(n int) string {
	parts := make([]string, n)
	for i := 0; i < n; i++ {
		parts[i] = fmt.Sprintf("line %d", i+1)
	}
	return strings.Join(parts, "\n")
}

func TestChunkDocument(t *testing.T) {
	doc := Document{Path: "main.go", Language: "go", Content: makeLines(130)}
	chunks := ChunkDocument("o/r", "sha", doc)

	// step = 60 - 10 = 50 → windows [1..60], [51..110], [101..130]
	if len(chunks) != 3 {
		t.Fatalf("got %d chunks, want 3", len(chunks))
	}
	if chunks[0].StartLine != 1 || chunks[0].EndLine != 60 {
		t.Errorf("chunk0 range = %d-%d, want 1-60", chunks[0].StartLine, chunks[0].EndLine)
	}
	if chunks[2].StartLine != 101 || chunks[2].EndLine != 130 {
		t.Errorf("chunk2 range = %d-%d, want 101-130", chunks[2].StartLine, chunks[2].EndLine)
	}
	for _, c := range chunks {
		if c.Repo != "o/r" || c.Path != "main.go" || c.Commit != "sha" {
			t.Errorf("chunk metadata wrong: %+v", c)
		}
	}
}

func TestChunkDocumentEmpty(t *testing.T) {
	if got := ChunkDocument("o/r", "sha", Document{Path: "x", Content: "   \n\n"}); got != nil {
		t.Errorf("expected nil for empty content, got %d chunks", len(got))
	}
}

// fakeEmbedder returns a fixed-dim vector per input (content-independent is fine
// for these tests since the fake store ignores similarity).
type fakeEmbedder struct{ calls int }

func (f *fakeEmbedder) Dim() int { return 3 }
func (f *fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	f.calls++
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{1, 0, 0}
	}
	return out, nil
}

// fakeStore is an in-memory VectorStore keyed by point ID.
type fakeStore struct {
	points  map[string]Point
	ensured bool
}

func newFakeStore() *fakeStore { return &fakeStore{points: map[string]Point{}} }

func (s *fakeStore) EnsureCollection(_ context.Context, _ int) error { s.ensured = true; return nil }
func (s *fakeStore) Upsert(_ context.Context, points []Point) error {
	for _, p := range points {
		s.points[p.ID] = p
	}
	return nil
}
func (s *fakeStore) Search(_ context.Context, repo string, _ []float32, k int) ([]Chunk, error) {
	var out []Chunk
	for _, p := range s.points {
		if p.Chunk.Repo == repo {
			out = append(out, p.Chunk)
		}
		if len(out) >= k {
			break
		}
	}
	return out, nil
}

func TestServiceIndexAndRetrieve(t *testing.T) {
	store := newFakeStore()
	svc := NewService(&fakeEmbedder{}, store, zerolog.Nop())

	if err := svc.EnsureReady(context.Background()); err != nil {
		t.Fatalf("EnsureReady: %v", err)
	}
	if !store.ensured {
		t.Error("EnsureCollection was not called")
	}

	docs := []Document{
		{Path: "a.go", Language: "go", Content: makeLines(70)}, // 2 chunks
		{Path: "b.go", Language: "go", Content: makeLines(20)}, // 1 chunk
	}
	n, err := svc.Index(context.Background(), "o/r", "sha", docs)
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if n != 2 {
		t.Errorf("indexed docs = %d, want 2", n)
	}
	if len(store.points) != 3 {
		t.Errorf("stored points = %d, want 3", len(store.points))
	}

	// Re-indexing the same docs must overwrite (stable IDs), not duplicate.
	if _, err := svc.Index(context.Background(), "o/r", "sha2", docs); err != nil {
		t.Fatalf("re-Index: %v", err)
	}
	if len(store.points) != 3 {
		t.Errorf("after re-index, points = %d, want 3 (stable IDs)", len(store.points))
	}

	got, err := svc.Retrieve(context.Background(), "o/r", "some query", 5)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(got) == 0 {
		t.Error("expected retrieved chunks, got none")
	}
}
