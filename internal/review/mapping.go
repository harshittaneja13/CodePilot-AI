package review

import (
	"path/filepath"
	"strings"

	"github.com/codepilot-ai/codepilot-ai/internal/models"
)

// mapPRData converts the raw GitHub PR data map into a PullRequest model.
func mapPRData(data map[string]interface{}, repoID string) *models.PullRequest {
	pr := &models.PullRequest{
		RepositoryID: repoID,
		State:        "open",
	}

	if num, ok := data["number"].(float64); ok {
		pr.GitHubNumber = int(num)
	}
	if title, ok := data["title"].(string); ok {
		pr.Title = title
	}
	if body, ok := data["body"].(string); ok {
		pr.BodyStr = body
	}
	if state, ok := data["state"].(string); ok {
		pr.State = state
	}
	if user, ok := data["user"].(map[string]interface{}); ok {
		if login, ok := user["login"].(string); ok {
			pr.Author = login
		}
	}
	if head, ok := data["head"].(map[string]interface{}); ok {
		if ref, ok := head["ref"].(string); ok {
			pr.HeadBranchStr = ref
		}
		if sha, ok := head["sha"].(string); ok {
			pr.HeadSHAStr = sha
		}
	}
	if base, ok := data["base"].(map[string]interface{}); ok {
		if ref, ok := base["ref"].(string); ok {
			pr.BaseBranchStr = ref
		}
	}
	if adds, ok := data["additions"].(float64); ok {
		pr.Additions = int(adds)
	}
	if dels, ok := data["deletions"].(float64); ok {
		pr.Deletions = int(dels)
	}
	if changed, ok := data["changed_files"].(float64); ok {
		pr.ChangedFiles = int(changed)
	}
	if url, ok := data["html_url"].(string); ok {
		pr.GitHubURLStr = url
	}

	return pr
}

// detectLanguage determines the programming language from a file extension.
func detectLanguage(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	langMap := map[string]string{
		".go":   "go",
		".py":   "python",
		".js":   "javascript",
		".ts":   "typescript",
		".tsx":  "typescript",
		".jsx":  "javascript",
		".java": "java",
		".rb":   "ruby",
		".rs":   "rust",
		".c":    "c",
		".cpp":  "cpp",
		".cc":   "cpp",
		".h":    "c",
		".hpp":  "cpp",
		".cs":   "csharp",
		".php":  "php",
		".sh":   "bash",
		".yaml": "yaml",
		".yml":  "yaml",
		".json": "json",
		".md":   "markdown",
		".sql":  "sql",
		".tf":   "terraform",
	}
	if lang, ok := langMap[ext]; ok {
		return lang
	}
	return "unknown"
}
