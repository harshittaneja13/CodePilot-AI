package rag

import "strings"

const (
	// defaultChunkLines is the window size (in lines) for each code chunk.
	defaultChunkLines = 60
	// defaultOverlap is how many lines consecutive chunks share, so a symbol that
	// straddles a boundary still appears whole in at least one chunk.
	defaultOverlap = 10
)

// Document is a source file to be indexed.
type Document struct {
	Path     string
	Language string
	Content  string
}

// Chunk is one indexed slice of a file, with the metadata needed to cite it back
// to the reviewer (path + line range).
type Chunk struct {
	Repo      string  `json:"repo"`
	Path      string  `json:"path"`
	Commit    string  `json:"commit"`
	Language  string  `json:"language"`
	StartLine int     `json:"start_line"`
	EndLine   int     `json:"end_line"`
	Content   string  `json:"content"`
	Score     float32 `json:"score,omitempty"` // similarity score, set on retrieval
}

// ChunkDocument splits a document into overlapping line-window chunks. Empty
// (whitespace-only) windows are skipped. Line numbers are 1-based and inclusive.
func ChunkDocument(repo, commit string, doc Document) []Chunk {
	if strings.TrimSpace(doc.Content) == "" {
		return nil
	}
	lines := strings.Split(doc.Content, "\n")
	step := defaultChunkLines - defaultOverlap
	if step < 1 {
		step = defaultChunkLines
	}

	var chunks []Chunk
	for start := 0; start < len(lines); start += step {
		end := start + defaultChunkLines
		if end > len(lines) {
			end = len(lines)
		}
		content := strings.Join(lines[start:end], "\n")
		if strings.TrimSpace(content) != "" {
			chunks = append(chunks, Chunk{
				Repo:      repo,
				Path:      doc.Path,
				Commit:    commit,
				Language:  doc.Language,
				StartLine: start + 1,
				EndLine:   end,
				Content:   content,
			})
		}
		if end == len(lines) {
			break
		}
	}
	return chunks
}
