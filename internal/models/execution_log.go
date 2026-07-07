package models

import (
	"database/sql"
	"encoding/json"
	"time"
)

// ExecutionLog records a step in the review processing pipeline for observability.
type ExecutionLog struct {
	ID          string          `json:"id"`
	ReviewID    string          `json:"review_id"`
	Step        string          `json:"step"`
	Status      string          `json:"status"`
	Message     sql.NullString  `json:"-"`
	MessageStr  string          `json:"message,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	DurationMs  int64           `json:"duration_ms"`
	CreatedAt   time.Time       `json:"created_at"`
}

// PopulateComputed fills exported JSON-facing fields from nullable DB columns.
func (el *ExecutionLog) PopulateComputed() {
	if el.Message.Valid {
		el.MessageStr = el.Message.String
	}
}

// CreateExecutionLogParams holds the parameters for creating a new execution log entry.
type CreateExecutionLogParams struct {
	ReviewID   string                 `json:"review_id"`
	Step       string                 `json:"step"`
	Status     string                 `json:"status"`
	Message    string                 `json:"message,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	DurationMs int64                  `json:"duration_ms"`
}
