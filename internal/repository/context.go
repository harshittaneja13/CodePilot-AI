// Package repository provides utilities for building repository context information.
package repository

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// MCPClient defines the interface for fetching file contents from GitHub.
type MCPClient interface {
	GetFileContents(ctx context.Context, owner, repo, path, ref string) (string, error)
}

// cacheEntry holds a cached context string with its expiration time.
type cacheEntry struct {
	content   string
	expiresAt time.Time
}

// ContextBuilder fetches and caches repository context documents.
type ContextBuilder struct {
	mcpClient MCPClient
	cache     map[string]cacheEntry
	mu        sync.RWMutex
	ttl       time.Duration
}

// NewContextBuilder creates a new ContextBuilder.
func NewContextBuilder(mcpClient MCPClient) *ContextBuilder {
	return &ContextBuilder{
		mcpClient: mcpClient,
		cache:     make(map[string]cacheEntry),
		ttl:       10 * time.Minute,
	}
}

// contextFiles lists the files to fetch for repository context, in priority order.
var contextFiles = []string{
	"README.md",
	"CONTRIBUTING.md",
	"docs/ARCHITECTURE.md",
}

// BuildContext fetches relevant repository documentation to provide context for code reviews.
// Results are cached in-memory with a 10-minute TTL.
func (cb *ContextBuilder) BuildContext(ctx context.Context, owner, repo, branch string) (string, error) {
	cacheKey := fmt.Sprintf("%s/%s@%s", owner, repo, branch)

	// Check cache first
	cb.mu.RLock()
	if entry, ok := cb.cache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		cb.mu.RUnlock()
		return entry.content, nil
	}
	cb.mu.RUnlock()

	// Fetch context files
	var sections []string

	for _, filePath := range contextFiles {
		content, err := cb.mcpClient.GetFileContents(ctx, owner, repo, filePath, branch)
		if err != nil {
			// Gracefully handle missing files
			continue
		}

		content = strings.TrimSpace(content)
		if content == "" {
			continue
		}

		// Truncate very large files to keep context manageable
		if len(content) > 4000 {
			content = content[:4000] + "\n... (truncated)"
		}

		sections = append(sections, fmt.Sprintf("### %s\n%s", filePath, content))
	}

	result := strings.Join(sections, "\n\n---\n\n")

	// Cache the result
	cb.mu.Lock()
	cb.cache[cacheKey] = cacheEntry{
		content:   result,
		expiresAt: time.Now().Add(cb.ttl),
	}
	cb.mu.Unlock()

	return result, nil
}
