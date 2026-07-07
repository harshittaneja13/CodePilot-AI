package models

import (
	"database/sql"
	"time"
)

// PullRequest represents a GitHub pull request tracked by CodePilot AI.
type PullRequest struct {
	ID           string         `json:"id"`
	RepositoryID string         `json:"repository_id"`
	GitHubNumber int            `json:"github_number"`
	Title        string         `json:"title"`
	Body         sql.NullString `json:"-"`
	BodyStr      string         `json:"body"`
	State        string         `json:"state"`
	Author       string         `json:"author"`
	HeadBranch   sql.NullString `json:"-"`
	HeadBranchStr string        `json:"head_branch"`
	BaseBranch   sql.NullString `json:"-"`
	BaseBranchStr string        `json:"base_branch"`
	HeadSHA      sql.NullString `json:"-"`
	HeadSHAStr   string         `json:"head_sha"`
	Additions    int            `json:"additions"`
	Deletions    int            `json:"deletions"`
	ChangedFiles int            `json:"changed_files"`
	GitHubURL    sql.NullString `json:"-"`
	GitHubURLStr string         `json:"github_url"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// PopulateComputed fills exported JSON-facing string fields from nullable DB columns.
func (pr *PullRequest) PopulateComputed() {
	if pr.Body.Valid {
		pr.BodyStr = pr.Body.String
	}
	if pr.HeadBranch.Valid {
		pr.HeadBranchStr = pr.HeadBranch.String
	}
	if pr.BaseBranch.Valid {
		pr.BaseBranchStr = pr.BaseBranch.String
	}
	if pr.HeadSHA.Valid {
		pr.HeadSHAStr = pr.HeadSHA.String
	}
	if pr.GitHubURL.Valid {
		pr.GitHubURLStr = pr.GitHubURL.String
	}
}

// PullRequestFile represents a single file changed in a pull request.
type PullRequestFile struct {
	Filename  string `json:"filename"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Patch     string `json:"patch"`
}

// CreatePullRequestParams holds the parameters for creating or upserting a pull request.
type CreatePullRequestParams struct {
	RepositoryID string `json:"repository_id"`
	GitHubNumber int    `json:"github_number"`
	Title        string `json:"title"`
	Body         string `json:"body"`
	State        string `json:"state"`
	Author       string `json:"author"`
	HeadBranch   string `json:"head_branch"`
	BaseBranch   string `json:"base_branch"`
	HeadSHA      string `json:"head_sha"`
	Additions    int    `json:"additions"`
	Deletions    int    `json:"deletions"`
	ChangedFiles int    `json:"changed_files"`
	GitHubURL    string `json:"github_url"`
}
