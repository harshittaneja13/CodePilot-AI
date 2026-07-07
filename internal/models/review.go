package models

import (
	"database/sql"
	"time"
)

// Review status constants matching the review_status PostgreSQL enum.
const (
	ReviewStatusPending    = "pending"
	ReviewStatusInProgress = "in_progress"
	ReviewStatusCompleted  = "completed"
	ReviewStatusFailed     = "failed"
)

// Severity level constants matching the severity_level PostgreSQL enum.
const (
	SeverityCritical = "critical"
	SeverityHigh     = "high"
	SeverityMedium   = "medium"
	SeverityLow      = "low"
)

// Review represents a code review performed by CodePilot AI.
type Review struct {
	ID               string         `json:"id"`
	PullRequestID    string         `json:"pull_request_id"`
	Status           string         `json:"status"`
	Summary          sql.NullString `json:"-"`
	SummaryStr       string         `json:"summary"`
	TotalComments    int            `json:"total_comments"`
	CriticalCount    int            `json:"critical_count"`
	HighCount        int            `json:"high_count"`
	MediumCount      int            `json:"medium_count"`
	LowCount         int            `json:"low_count"`
	LLMModel         sql.NullString `json:"-"`
	LLMModelStr      string         `json:"llm_model"`
	TokensUsed       int            `json:"tokens_used"`
	InputTokens      int            `json:"input_tokens"`
	OutputTokens     int            `json:"output_tokens"`
	CostUSD          float64        `json:"cost_usd"`
	ProcessingTimeMs int64          `json:"processing_time_ms"`
	ErrorMessage     sql.NullString `json:"-"`
	ErrorMessageStr  string         `json:"error_message,omitempty"`
	StartedAt        sql.NullTime   `json:"-"`
	StartedAtPtr     *time.Time     `json:"started_at"`
	CompletedAt      sql.NullTime   `json:"-"`
	CompletedAtPtr   *time.Time     `json:"completed_at"`
	CreatedAt        time.Time      `json:"created_at"`
}

// PopulateComputed fills exported JSON-facing fields from nullable DB columns.
func (r *Review) PopulateComputed() {
	if r.Summary.Valid {
		r.SummaryStr = r.Summary.String
	}
	if r.LLMModel.Valid {
		r.LLMModelStr = r.LLMModel.String
	}
	if r.ErrorMessage.Valid {
		r.ErrorMessageStr = r.ErrorMessage.String
	}
	if r.StartedAt.Valid {
		t := r.StartedAt.Time
		r.StartedAtPtr = &t
	}
	if r.CompletedAt.Valid {
		t := r.CompletedAt.Time
		r.CompletedAtPtr = &t
	}
}

// ReviewComment represents a single comment within a code review.
type ReviewComment struct {
	ID           string         `json:"id"`
	ReviewID     string         `json:"review_id"`
	FilePath     string         `json:"file_path"`
	LineNumber   sql.NullInt32  `json:"-"`
	LineNumberVal *int          `json:"line_number"`
	Severity     string         `json:"severity"`
	Title        string         `json:"title"`
	Explanation  string         `json:"explanation"`
	WhyItMatters sql.NullString `json:"-"`
	WhyItMattersStr string      `json:"why_it_matters,omitempty"`
	Suggestion   sql.NullString `json:"-"`
	SuggestionStr string        `json:"suggestion,omitempty"`
	CodeSnippet  sql.NullString `json:"-"`
	CodeSnippetStr string       `json:"code_snippet,omitempty"`
	Published    bool           `json:"published"`
	CreatedAt    time.Time      `json:"created_at"`
}

// PopulateComputed fills exported JSON-facing fields from nullable DB columns.
func (rc *ReviewComment) PopulateComputed() {
	if rc.LineNumber.Valid {
		v := int(rc.LineNumber.Int32)
		rc.LineNumberVal = &v
	}
	if rc.WhyItMatters.Valid {
		rc.WhyItMattersStr = rc.WhyItMatters.String
	}
	if rc.Suggestion.Valid {
		rc.SuggestionStr = rc.Suggestion.String
	}
	if rc.CodeSnippet.Valid {
		rc.CodeSnippetStr = rc.CodeSnippet.String
	}
}

// ReviewWithComments aggregates a review and its associated comments.
type ReviewWithComments struct {
	Review   Review          `json:"review"`
	Comments []ReviewComment `json:"comments"`
}

// CreateReviewParams holds the parameters for creating a new review.
type CreateReviewParams struct {
	PullRequestID string `json:"pull_request_id"`
	LLMModel      string `json:"llm_model,omitempty"`
}

// CreateReviewCommentParams holds the parameters for adding a comment to a review.
type CreateReviewCommentParams struct {
	ReviewID     string `json:"review_id"`
	FilePath     string `json:"file_path"`
	LineNumber   *int   `json:"line_number"`
	Severity     string `json:"severity"`
	Title        string `json:"title"`
	Explanation  string `json:"explanation"`
	WhyItMatters string `json:"why_it_matters,omitempty"`
	Suggestion   string `json:"suggestion,omitempty"`
	CodeSnippet  string `json:"code_snippet,omitempty"`
}
