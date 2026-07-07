package review

import (
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/codepilot-ai/codepilot-ai/internal/llm"
)

var hunkHeader = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

// DiffCommentLines returns RIGHT-side line numbers accepted by GitHub for an
// inline review comment. Context and added lines are commentable; deleted lines
// are not because they belong to the LEFT side.
func DiffCommentLines(patch string) map[int]struct{} {
	result := make(map[int]struct{})
	newLine := 0
	insideHunk := false
	for _, line := range strings.Split(patch, "\n") {
		if match := hunkHeader.FindStringSubmatch(line); len(match) == 2 {
			newLine, _ = strconv.Atoi(match[1])
			insideHunk = true
			continue
		}
		if !insideHunk || strings.HasPrefix(line, "\\ No newline") {
			continue
		}
		switch {
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			result[newLine] = struct{}{}
			newLine++
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			// A deletion has no RIGHT-side line.
		default:
			result[newLine] = struct{}{}
			newLine++
		}
	}
	return result
}

func splitUnifiedDiff(diff string) map[string]string {
	result := make(map[string]string)
	var currentPath string
	var current strings.Builder
	flush := func() {
		if currentPath != "" {
			result[currentPath] = strings.TrimSpace(current.String())
		}
		current.Reset()
	}
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git a/") {
			flush()
			parts := strings.SplitN(strings.TrimPrefix(line, "diff --git a/"), " b/", 2)
			if len(parts) == 2 {
				currentPath = parts[1]
			}
		}
		if currentPath != "" {
			current.WriteString(line)
			current.WriteByte('\n')
		}
	}
	flush()
	return result
}

func filterPublishableFindings(findings []llm.ReviewFinding, patches map[string]string) []llm.ReviewFinding {
	lineCache := make(map[string]map[int]struct{}, len(patches))
	result := make([]llm.ReviewFinding, 0, len(findings))
	for _, finding := range findings {
		patch, ok := patches[finding.FilePath]
		if !ok || finding.LineNumber <= 0 {
			continue
		}
		lines, ok := lineCache[finding.FilePath]
		if !ok {
			lines = DiffCommentLines(patch)
			lineCache[finding.FilePath] = lines
		}
		if _, ok := lines[finding.LineNumber]; ok {
			result = append(result, finding)
		}
	}
	return result
}

func isExcluded(filePath string, patterns []string) bool {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if strings.HasSuffix(pattern, "/**") && strings.HasPrefix(filePath, strings.TrimSuffix(pattern, "**")) {
			return true
		}
		if matched, _ := path.Match(pattern, filePath); matched {
			return true
		}
		if matched, _ := path.Match(pattern, path.Base(filePath)); matched {
			return true
		}
	}
	return false
}

func removePreviouslyPublished(findings []llm.ReviewFinding, existing interface{}) []llm.ReviewFinding {
	existingText := strings.ToLower(strings.Join(flattenStrings(existing), " "))
	if existingText == "" {
		return findings
	}
	result := make([]llm.ReviewFinding, 0, len(findings))
	for _, finding := range findings {
		title := strings.ToLower(strings.TrimSpace(finding.Title))
		if title != "" && strings.Contains(existingText, title) {
			continue
		}
		result = append(result, finding)
	}
	return result
}

func flattenStrings(value interface{}) []string {
	var result []string
	switch typed := value.(type) {
	case string:
		result = append(result, typed)
	case []map[string]interface{}:
		for _, item := range typed {
			result = append(result, flattenStrings(item)...)
		}
	case []interface{}:
		for _, item := range typed {
			result = append(result, flattenStrings(item)...)
		}
	case map[string]interface{}:
		for _, item := range typed {
			result = append(result, flattenStrings(item)...)
		}
	}
	return result
}
