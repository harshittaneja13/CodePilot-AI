package analyzer

import (
	"strings"
)

// CalculateCyclomaticComplexity counts decision points in source code
// to produce an approximate cyclomatic complexity score.
// The baseline complexity is 1 (a function with no branches).
func CalculateCyclomaticComplexity(code string, language string) int {
	complexity := 1
	lines := strings.Split(code, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip comments.
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") ||
			strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "/*") {
			continue
		}

		switch language {
		case "go":
			complexity += countGoDecisionPoints(trimmed)
		case "python":
			complexity += countPythonDecisionPoints(trimmed)
		case "javascript", "typescript":
			complexity += countJSDecisionPoints(trimmed)
		case "java":
			complexity += countJavaDecisionPoints(trimmed)
		default:
			// Generic: count common keywords.
			complexity += countGenericDecisionPoints(trimmed)
		}
	}

	return complexity
}

// countGoDecisionPoints counts Go-specific decision points.
func countGoDecisionPoints(line string) int {
	count := 0
	tokens := tokenize(line)

	for _, tok := range tokens {
		switch tok {
		case "if", "else", "case", "for", "select":
			count++
		}
	}

	// Boolean operators create additional paths.
	count += strings.Count(line, "&&")
	count += strings.Count(line, "||")

	return count
}

// countPythonDecisionPoints counts Python-specific decision points.
func countPythonDecisionPoints(line string) int {
	count := 0
	tokens := tokenize(line)

	for _, tok := range tokens {
		switch tok {
		case "if", "elif", "else", "for", "while", "except", "with":
			count++
		}
	}

	count += strings.Count(line, " and ")
	count += strings.Count(line, " or ")
	// Ternary: `x if cond else y` — already counted by if/else tokens.

	return count
}

// countJSDecisionPoints counts JavaScript/TypeScript-specific decision points.
func countJSDecisionPoints(line string) int {
	count := 0
	tokens := tokenize(line)

	for _, tok := range tokens {
		switch tok {
		case "if", "else", "case", "for", "while", "catch":
			count++
		}
	}

	count += strings.Count(line, "&&")
	count += strings.Count(line, "||")
	count += strings.Count(line, "??") // nullish coalescing
	// Ternary operator.
	count += strings.Count(line, "?")
	// Subtract back the nullish coalescing already counted.
	// Actually `?` in ternary and `??` overlap, handle carefully:
	// We want to count `?` but not when it's part of `??` or `?.` (optional chaining).
	ternaryCount := 0
	for i := 0; i < len(line); i++ {
		if line[i] == '?' {
			if i+1 < len(line) && (line[i+1] == '?' || line[i+1] == '.') {
				i++ // skip `??` or `?.`
				continue
			}
			ternaryCount++
		}
	}
	// Reset and use precise ternary count.
	count -= strings.Count(line, "?")
	count += ternaryCount

	return count
}

// countJavaDecisionPoints counts Java-specific decision points.
func countJavaDecisionPoints(line string) int {
	count := 0
	tokens := tokenize(line)

	for _, tok := range tokens {
		switch tok {
		case "if", "else", "case", "for", "while", "catch":
			count++
		}
	}

	count += strings.Count(line, "&&")
	count += strings.Count(line, "||")
	// Ternary operator (simple count).
	for i := 0; i < len(line); i++ {
		if line[i] == '?' && (i+1 >= len(line) || line[i+1] != '.') {
			count++
		}
	}

	return count
}

// countGenericDecisionPoints handles unknown languages with common keywords.
func countGenericDecisionPoints(line string) int {
	count := 0
	tokens := tokenize(line)

	for _, tok := range tokens {
		switch tok {
		case "if", "else", "elif", "case", "for", "while", "catch", "except":
			count++
		}
	}

	count += strings.Count(line, "&&")
	count += strings.Count(line, "||")

	return count
}

// tokenize splits a line into whitespace-separated tokens,
// stripping common punctuation so that keywords at the start of
// statements (e.g., "if(") are correctly identified.
func tokenize(line string) []string {
	// Replace common prefix/suffix characters that might be glued to keywords.
	replacer := strings.NewReplacer(
		"(", " ", ")", " ",
		"{", " ", "}", " ",
		":", " ", ";", " ",
		",", " ",
	)
	cleaned := replacer.Replace(line)
	return strings.Fields(cleaned)
}
