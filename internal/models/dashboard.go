package models

import "time"

// ReviewActivity represents review volume for a single day.
type ReviewActivity struct {
	Date     string `json:"date"`
	Reviews  int    `json:"reviews"`
	Findings int    `json:"findings"`
}

// DashboardReview is a flattened view of a review including its PR and repository
// context, suitable for list displays on the dashboard.
type DashboardReview struct {
	ID               string     `json:"id"`
	PullRequestID    string     `json:"pull_request_id"`
	RepositoryID     string     `json:"repository_id"`
	RepositoryName   string     `json:"repository_name"`
	PRNumber         int        `json:"pr_number"`
	PRTitle          string     `json:"pr_title"`
	Status           string     `json:"status"`
	LLMModel         string     `json:"llm_model"`
	TotalFindings    int        `json:"total_findings"`
	CriticalCount    int        `json:"critical_count"`
	HighCount        int        `json:"high_count"`
	MediumCount      int        `json:"medium_count"`
	LowCount         int        `json:"low_count"`
	TokensUsed       int        `json:"tokens_used"`
	CostUSD          float64    `json:"cost_usd"`
	ProcessingTimeMs int64      `json:"processing_time_ms"`
	ErrorMessage     string     `json:"error_message,omitempty"`
	StartedAt        *time.Time `json:"started_at"`
	CompletedAt      *time.Time `json:"completed_at"`
	CreatedAt        time.Time  `json:"created_at"`
}

// ActivityEvent is a single pipeline milestone (an execution-log entry) enriched with
// its review/PR/repository context, for the dashboard's live activity feed.
type ActivityEvent struct {
	ID             string    `json:"id"`
	ReviewID       string    `json:"review_id"`
	PullRequestID  string    `json:"pull_request_id"`
	RepositoryName string    `json:"repository_name"`
	PRNumber       int       `json:"pr_number"`
	PRTitle        string    `json:"pr_title"`
	Step           string    `json:"step"`
	Status         string    `json:"status"`
	Message        string    `json:"message,omitempty"`
	DurationMs     int64     `json:"duration_ms"`
	CreatedAt      time.Time `json:"created_at"`
}

// DashboardStats provides an aggregate overview for the CodePilot AI dashboard.
type DashboardStats struct {
	TotalRepositories  int              `json:"total_repositories"`
	TotalPullRequests  int              `json:"total_pull_requests"`
	TotalReviews       int              `json:"total_reviews"`
	TotalFindings      int              `json:"total_findings"`
	CriticalFindings   int              `json:"critical_findings"`
	HighFindings       int              `json:"high_findings"`
	MediumFindings     int              `json:"medium_findings"`
	LowFindings        int              `json:"low_findings"`
	ReviewsToday       int              `json:"reviews_today"`
	ReviewsThisWeek    int              `json:"reviews_this_week"`
	TotalCostUSD       float64          `json:"total_cost_usd"`
	AvgReviewTimeMs    int64            `json:"average_review_time_ms"`
	ReviewActivity     []ReviewActivity `json:"review_activity"`
	RecentReviews      []DashboardReview `json:"recent_reviews"`
}
