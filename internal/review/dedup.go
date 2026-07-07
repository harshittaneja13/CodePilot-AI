package review

import (
	"strings"

	"github.com/codepilot-ai/codepilot-ai/internal/llm"
)

// DeduplicateFindings removes duplicate findings based on file path, line number,
// and title similarity. Keeps the first occurrence.
func DeduplicateFindings(findings []llm.ReviewFinding) []llm.ReviewFinding {
	if len(findings) == 0 {
		return findings
	}

	type key struct {
		filePath   string
		lineNumber int
	}

	seen := make(map[key][]string) // maps (file, line) to list of seen titles
	var deduped []llm.ReviewFinding

	for _, f := range findings {
		k := key{filePath: f.FilePath, lineNumber: f.LineNumber}

		if existingTitles, ok := seen[k]; ok {
			duplicate := false
			for _, existingTitle := range existingTitles {
				if titlesAreSimilar(existingTitle, f.Title) {
					duplicate = true
					break
				}
			}
			if duplicate {
				continue
			}
		}

		seen[k] = append(seen[k], f.Title)
		deduped = append(deduped, f)
	}

	return deduped
}

// titlesAreSimilar checks if two titles are similar enough to be considered duplicates.
// Uses case-insensitive string containment.
func titlesAreSimilar(a, b string) bool {
	aLower := strings.ToLower(a)
	bLower := strings.ToLower(b)

	if aLower == bLower {
		return true
	}

	// Check if one title contains the other
	if strings.Contains(aLower, bLower) || strings.Contains(bLower, aLower) {
		return true
	}

	return false
}
