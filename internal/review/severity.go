package review

import (
	"sort"
	"strings"

	"github.com/codepilot-ai/codepilot-ai/internal/llm"
	"github.com/codepilot-ai/codepilot-ai/internal/models"
)

// securityPaths are path fragments that suggest security-sensitive code.
var securityPaths = []string{
	"auth", "login", "oauth", "token", "session", "password", "secret",
	"crypto", "encrypt", "jwt", "permission", "role", "access", "middleware",
	"payment", "billing", "checkout", "webhook",
}

// lowPriorityExtensions are file types that rarely contain critical logic.
var lowPriorityExtensions = []string{
	".md", ".txt", ".json", ".yaml", ".yml", ".toml", ".lock",
	".svg", ".png", ".jpg", ".gif", ".ico",
}

// filePriority returns a lower number for higher-priority files.
// Used to sort files before the triage token-budget cap so that when we must
// truncate to maxTriageFiles, the most important files are always kept.
func filePriority(f llm.FileContext) int {
	path := strings.ToLower(f.Path)
	ext := strings.ToLower(f.Language)

	// Tier 0: security-sensitive paths
	for _, kw := range securityPaths {
		if strings.Contains(path, kw) {
			return 0
		}
	}

	// Tier 1: source code with significant changes
	switch ext {
	case "go", "python", "typescript", "javascript", "java", "rust", "csharp", "php", "ruby":
		if f.Additions+f.Deletions > 20 {
			return 1
		}
		return 2
	case "sql":
		return 1 // schema/query changes are always important
	}

	// Tier 2: config / infrastructure
	switch ext {
	case "yaml", "terraform", "bash":
		return 3
	}

	// Tier 3: low-priority types
	for _, suf := range lowPriorityExtensions {
		if strings.HasSuffix(path, suf) {
			return 5
		}
	}

	return 4
}

// SortByPriority sorts file contexts so the most review-worthy files come first.
// Security-sensitive paths → high-churn source files → config → docs/assets.
func SortByPriority(files []llm.FileContext) []llm.FileContext {
	sorted := make([]llm.FileContext, len(files))
	copy(sorted, files)
	sort.SliceStable(sorted, func(i, j int) bool {
		pi, pj := filePriority(sorted[i]), filePriority(sorted[j])
		if pi != pj {
			return pi < pj
		}
		// Within the same tier, prefer files with more changes
		return sorted[i].Additions+sorted[i].Deletions > sorted[j].Additions+sorted[j].Deletions
	})
	return sorted
}

// validSeverities is the set of valid severity levels.
var validSeverities = map[string]bool{
	models.SeverityCritical: true,
	models.SeverityHigh:     true,
	models.SeverityMedium:   true,
	models.SeverityLow:      true,
}

// ValidateSeverity returns the severity if valid, or "medium" as a safe default.
func ValidateSeverity(s string) string {
	lower := strings.ToLower(strings.TrimSpace(s))
	if validSeverities[lower] {
		return lower
	}
	return models.SeverityMedium
}

// AdjustSeverities applies domain-specific severity rules:
// - Security-related findings must be at least "high"
// - Style/formatting findings are capped at "medium"
func AdjustSeverities(findings []llm.ReviewFinding) {
	securityKeywords := []string{
		"security", "vulnerability", "injection", "xss", "csrf",
		"secret", "password", "credential", "authentication",
		"authorization", "privilege", "exploit", "backdoor",
	}

	styleKeywords := []string{
		"style", "formatting", "naming", "convention", "whitespace",
		"indentation", "spacing", "lint", "cosmetic",
	}

	for i := range findings {
		findings[i].Severity = ValidateSeverity(findings[i].Severity)

		titleLower := strings.ToLower(findings[i].Title)
		explanationLower := strings.ToLower(findings[i].Explanation)
		combined := titleLower + " " + explanationLower

		// Security findings must be at least "high"
		if containsAny(combined, securityKeywords) {
			if findings[i].Severity == models.SeverityMedium || findings[i].Severity == models.SeverityLow {
				findings[i].Severity = models.SeverityHigh
			}
		}

		// Style findings capped at "medium"
		if containsAny(combined, styleKeywords) {
			if findings[i].Severity == models.SeverityCritical || findings[i].Severity == models.SeverityHigh {
				findings[i].Severity = models.SeverityMedium
			}
		}
	}
}

// CountBySeverity returns a map of severity level to count.
func CountBySeverity(findings []llm.ReviewFinding) map[string]int {
	counts := map[string]int{
		models.SeverityCritical: 0,
		models.SeverityHigh:     0,
		models.SeverityMedium:   0,
		models.SeverityLow:      0,
	}

	for _, f := range findings {
		sev := ValidateSeverity(f.Severity)
		counts[sev]++
	}

	return counts
}

// FilterByConfidence removes findings whose confidence score is below the given threshold.
// Findings with confidence == 0 (field absent from LLM output) are kept unchanged.
func FilterByConfidence(findings []llm.ReviewFinding, threshold float64) (kept, removed []llm.ReviewFinding) {
	for _, f := range findings {
		if f.Confidence > 0 && f.Confidence < threshold {
			removed = append(removed, f)
		} else {
			kept = append(kept, f)
		}
	}
	return kept, removed
}

// ApplyReflectionValidations merges reflection-pass results back into the findings slice.
// Each entry in validations corresponds to a finding by index. Findings marked invalid
// or with a post-reflection confidence below threshold are moved to the removed slice.
func ApplyReflectionValidations(findings []llm.ReviewFinding, validations []llm.FindingValidation, threshold float64) (kept, removed []llm.ReviewFinding) {
	validationMap := make(map[int]llm.FindingValidation, len(validations))
	for _, v := range validations {
		validationMap[v.Index] = v
	}
	for i, f := range findings {
		v, hasVal := validationMap[i]
		if !hasVal {
			// No validation returned for this index — keep it
			kept = append(kept, f)
			continue
		}
		// Update confidence from reflection LLM's independent assessment
		if v.Confidence > 0 {
			f.Confidence = v.Confidence
		}
		if !v.IsValid || (v.Confidence > 0 && v.Confidence < threshold) {
			removed = append(removed, f)
		} else {
			kept = append(kept, f)
		}
	}
	return kept, removed
}

// containsAny checks if the text contains any of the given keywords.
func containsAny(text string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}
