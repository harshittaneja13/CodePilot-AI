// Package llm provides types and client functionality for interacting with LLM providers.
package llm

// ChatMessage represents a single message in a chat conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents a request to the LLM chat completions API.
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
}

// ChatResponse represents the response from the LLM.
type ChatResponse struct {
	Content      string `json:"content"`
	TokensUsed   int    `json:"tokens_used"` // total (input + output)
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	Model        string `json:"model"`
}

// ReviewFinding represents a single finding from the LLM code review.
type ReviewFinding struct {
	FilePath     string  `json:"file_path"`
	LineNumber   int     `json:"line_number"`
	Title        string  `json:"title"`
	Explanation  string  `json:"explanation"`
	WhyItMatters string  `json:"why_it_matters"`
	Suggestion   string  `json:"suggestion"`
	Severity     string  `json:"severity"`
	CodeSnippet  string  `json:"code_snippet"`
	Confidence   float64 `json:"confidence"` // 0.0–1.0; findings below 0.7 are filtered by the reflection pass
}

// ReviewResult holds the parsed output from the LLM review.
type ReviewResult struct {
	Summary  string          `json:"summary"`
	Findings []ReviewFinding `json:"findings"`
}

// TriageResult holds the output of the triage LLM call (phase 1).
// The LLM decides which files are security/logic-critical and worth deep review.
type TriageResult struct {
	PriorityFiles []string `json:"priority_files"`
	Rationale     string   `json:"rationale"`
}

// FindingValidation is one entry in the reflection pass output (phase 3).
type FindingValidation struct {
	Index      int     `json:"index"`
	IsValid    bool    `json:"is_valid"`
	Confidence float64 `json:"confidence"` // independent confidence score from the reflection LLM
	Reason     string  `json:"reason"`
}

// ValidationResult holds the output of the reflection LLM call (phase 3).
type ValidationResult struct {
	Validations []FindingValidation `json:"validations"`
}
