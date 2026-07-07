package models

import (
	"database/sql"
	"encoding/json"
	"time"
)

// Repository represents a GitHub repository tracked by CodePilot AI.
type Repository struct {
	ID            string              `json:"id"`
	GitHubID      int64               `json:"github_id"`
	Owner         string              `json:"owner"`
	Name          string              `json:"name"`
	FullName      string              `json:"full_name"`
	Description   sql.NullString      `json:"-"`
	DescriptionStr string             `json:"description"`
	DefaultBranch string              `json:"default_branch"`
	Language      sql.NullString      `json:"-"`
	LanguageStr   string              `json:"language"`
	IsActive      bool                `json:"is_active"`
	WebhookID     sql.NullInt64       `json:"-"`
	WebhookIDVal  *int64              `json:"webhook_id"`
	Settings      RepositorySettings  `json:"settings"`
	SettingsRaw   json.RawMessage     `json:"-"`
	CreatedAt     time.Time           `json:"created_at"`
	UpdatedAt     time.Time           `json:"updated_at"`
}

// PopulateComputed fills computed JSON fields from nullable database fields.
func (r *Repository) PopulateComputed() {
	if r.Description.Valid {
		r.DescriptionStr = r.Description.String
	}
	if r.Language.Valid {
		r.LanguageStr = r.Language.String
	}
	if r.WebhookID.Valid {
		val := r.WebhookID.Int64
		r.WebhookIDVal = &val
	}
	r.Settings = DefaultRepositorySettings()
	if len(r.SettingsRaw) > 0 {
		_ = json.Unmarshal(r.SettingsRaw, &r.Settings)
	}
}

// DefaultRepositorySettings are applied when a repository has not overridden
// a review option. Auto-review is enabled for newly connected repositories.
func DefaultRepositorySettings() RepositorySettings {
	return RepositorySettings{
		AutoReview:        true,
		ReviewOnDraft:     false,
		ExcludePatterns:   []string{"vendor/**", "node_modules/**", "dist/**", "*.min.js", "*.lock"},
		MaxFilesPerReview: 50,
	}
}

// RepositorySettings holds configurable settings stored as JSONB in the database.
type RepositorySettings struct {
	LLMModel          string   `json:"llm_model,omitempty"`
	AutoReview        bool     `json:"auto_review"`
	ReviewOnDraft     bool     `json:"review_on_draft"`
	ExcludePatterns   []string `json:"exclude_patterns,omitempty"`
	MaxFilesPerReview int      `json:"max_files_per_review,omitempty"`
}

// CreateRepositoryRequest is the DTO for creating a new repository.
type CreateRepositoryRequest struct {
	GitHubID      int64  `json:"github_id"`
	Owner         string `json:"owner" binding:"required"`
	Name          string `json:"name" binding:"required"`
	FullName      string `json:"full_name"`
	Description   string `json:"description"`
	DefaultBranch string `json:"default_branch"`
	Language      string `json:"language"`
}

// UpdateRepositorySettingsRequest is the DTO for updating repository settings.
type UpdateRepositorySettingsRequest struct {
	LLMModel          *string  `json:"llm_model"`
	AutoReview        *bool    `json:"auto_review"`
	ReviewOnDraft     *bool    `json:"review_on_draft"`
	ExcludePatterns   []string `json:"exclude_patterns"`
	MaxFilesPerReview *int     `json:"max_files_per_review"`
}
