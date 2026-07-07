// Package rag provides retrieval-augmented context for code review: it chunks and
// embeds repository files into a Qdrant vector store and retrieves the chunks most
// relevant to a query. The review agent uses it to see cross-file context (a called
// function's definition, related code) beyond the raw diff.
package rag

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Retriever returns code chunks relevant to a query, scoped to a repository.
type Retriever interface {
	Retrieve(ctx context.Context, repo, query string, k int) ([]Chunk, error)
}

// Service ties an Embedder to a VectorStore to provide indexing and retrieval.
type Service struct {
	embedder Embedder
	store    VectorStore
	logger   zerolog.Logger
}

// NewService constructs a RAG service.
func NewService(embedder Embedder, store VectorStore, logger zerolog.Logger) *Service {
	return &Service{
		embedder: embedder,
		store:    store,
		logger:   logger.With().Str("component", "rag").Logger(),
	}
}

// EnsureReady creates the vector collection if it does not yet exist.
func (s *Service) EnsureReady(ctx context.Context) error {
	return s.store.EnsureCollection(ctx, s.embedder.Dim())
}

// Index chunks the documents, embeds them, and upserts them into the store.
// It returns the number of documents indexed.
func (s *Service) Index(ctx context.Context, repo, commit string, docs []Document) (int, error) {
	var chunks []Chunk
	for _, d := range docs {
		chunks = append(chunks, ChunkDocument(repo, commit, d)...)
	}
	if len(chunks) == 0 {
		return 0, nil
	}

	texts := make([]string, len(chunks))
	for i, c := range chunks {
		texts[i] = c.Content
	}
	vecs, err := s.embedder.Embed(ctx, texts)
	if err != nil {
		return 0, fmt.Errorf("embedding chunks: %w", err)
	}
	if len(vecs) != len(chunks) {
		return 0, fmt.Errorf("embedding count mismatch: %d vectors for %d chunks", len(vecs), len(chunks))
	}

	points := make([]Point, len(chunks))
	for i, c := range chunks {
		points[i] = Point{ID: pointID(c), Vector: vecs[i], Chunk: c}
	}
	if err := s.store.Upsert(ctx, points); err != nil {
		return 0, err
	}
	return len(docs), nil
}

// Retrieve embeds the query and returns the top-k most similar chunks in the repo.
func (s *Service) Retrieve(ctx context.Context, repo, query string, k int) ([]Chunk, error) {
	if k <= 0 {
		k = 5
	}
	vecs, err := s.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embedding query: %w", err)
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("no embedding returned for query")
	}
	return s.store.Search(ctx, repo, vecs[0], k)
}

// pointID derives a stable UUID for a chunk from repo+path+start line so that
// re-indexing the same location overwrites the point instead of duplicating it.
func pointID(c Chunk) string {
	key := strings.Join([]string{c.Repo, c.Path, fmt.Sprint(c.StartLine)}, "|")
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(key)).String()
}
