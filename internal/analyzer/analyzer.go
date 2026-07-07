package analyzer

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog"
)

// FileInput represents a file to be analyzed.
type FileInput struct {
	Path     string
	Content  string
	Language string
}

// AnalysisIssue represents a single issue found during static analysis.
type AnalysisIssue struct {
	Line     int    `json:"line"`
	Message  string `json:"message"`
	Rule     string `json:"rule"`
	Severity string `json:"severity"` // "critical", "high", "medium", "low"
}

// FileAnalysis holds analysis results for a single file.
type FileAnalysis struct {
	Path       string          `json:"path"`
	Issues     []AnalysisIssue `json:"issues"`
	Complexity int             `json:"complexity"`
}

// AnalysisResults holds analysis results across all files.
type AnalysisResults struct {
	Files []FileAnalysis `json:"files"`
}

// Analyzer performs deterministic static analysis on source code.
type Analyzer struct {
	securityPatterns []Pattern
	qualityPatterns  []Pattern
	logger          zerolog.Logger
}

// NewAnalyzer creates a new Analyzer with precompiled patterns.
func NewAnalyzer(logger zerolog.Logger) *Analyzer {
	return &Analyzer{
		securityPatterns: SecurityPatterns(),
		qualityPatterns:  QualityPatterns(),
		logger:          logger.With().Str("component", "analyzer").Logger(),
	}
}

// AnalyzeFiles runs all deterministic checks across the given files.
func (a *Analyzer) AnalyzeFiles(ctx context.Context, files []FileInput) (*AnalysisResults, error) {
	results := &AnalysisResults{
		Files: make([]FileAnalysis, 0, len(files)),
	}

	for _, f := range files {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("analysis canceled: %w", ctx.Err())
		default:
		}

		fa := a.analyzeFile(f)
		results.Files = append(results.Files, fa)

		a.logger.Debug().
			Str("path", f.Path).
			Int("issues", len(fa.Issues)).
			Int("complexity", fa.Complexity).
			Msg("file analyzed")
	}

	return results, nil
}

// analyzeFile runs all checks on a single file.
func (a *Analyzer) analyzeFile(f FileInput) FileAnalysis {
	fa := FileAnalysis{
		Path:   f.Path,
		Issues: make([]AnalysisIssue, 0),
	}

	lines := strings.Split(f.Content, "\n")

	// Cyclomatic complexity.
	fa.Complexity = CalculateCyclomaticComplexity(f.Content, f.Language)

	// Large file detection.
	if len(lines) > 500 {
		fa.Issues = append(fa.Issues, AnalysisIssue{
			Line:     1,
			Message:  fmt.Sprintf("File has %d lines; consider breaking it into smaller modules", len(lines)),
			Rule:     "large-file",
			Severity: "low",
		})
	}

	// Pattern-based checks.
	for _, p := range a.securityPatterns {
		fa.Issues = append(fa.Issues, matchPattern(p, lines)...)
	}
	for _, p := range a.qualityPatterns {
		fa.Issues = append(fa.Issues, matchPattern(p, lines)...)
	}

	// Structural checks.
	fa.Issues = append(fa.Issues, detectLongFunctions(lines, f.Language)...)
	fa.Issues = append(fa.Issues, detectDeepNesting(lines, f.Language)...)
	fa.Issues = append(fa.Issues, detectTodoComments(lines)...)
	fa.Issues = append(fa.Issues, detectMagicNumbers(lines, f.Language)...)
	fa.Issues = append(fa.Issues, detectUncheckedErrors(lines, f.Language)...)
	fa.Issues = append(fa.Issues, detectPotentialNilPointer(lines, f.Language)...)
	fa.Issues = append(fa.Issues, detectUnusedParameters(lines, f.Language)...)

	return fa
}

// matchPattern runs a single regex pattern against all lines.
func matchPattern(p Pattern, lines []string) []AnalysisIssue {
	var issues []AnalysisIssue
	for i, line := range lines {
		if p.Compiled != nil && p.Compiled.MatchString(line) {
			issues = append(issues, AnalysisIssue{
				Line:     i + 1,
				Message:  p.Message,
				Rule:     p.Name,
				Severity: p.Severity,
			})
		}
	}
	return issues
}

// detectLongFunctions finds functions longer than 50 lines.
func detectLongFunctions(lines []string, language string) []AnalysisIssue {
	var issues []AnalysisIssue

	type funcInfo struct {
		name      string
		startLine int
		braceDepth int
	}

	var current *funcInfo

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		switch language {
		case "go", "java", "javascript", "typescript", "c", "cpp":
			// Detect function start: line contains "func " (Go) or common patterns, and ends with "{"
			isFuncStart := false
			funcName := ""

			if language == "go" && strings.Contains(trimmed, "func ") && strings.HasSuffix(trimmed, "{") {
				isFuncStart = true
				parts := strings.Fields(trimmed)
				for j, p := range parts {
					if p == "func" && j+1 < len(parts) {
						funcName = parts[j+1]
						// Strip receiver
						if strings.HasPrefix(funcName, "(") {
							for k := j + 2; k < len(parts); k++ {
								if !strings.HasSuffix(parts[k], ")") {
									continue
								}
								if k+1 < len(parts) {
									funcName = parts[k+1]
								}
								break
							}
						}
						break
					}
				}
			} else if language != "go" && (strings.Contains(trimmed, "function ") || strings.Contains(trimmed, "=> {")) && strings.HasSuffix(trimmed, "{") {
				isFuncStart = true
				funcName = trimmed
			}

			if isFuncStart && current == nil {
				current = &funcInfo{
					name:      funcName,
					startLine: i + 1,
					braceDepth: 1,
				}
				continue
			}

			if current != nil {
				current.braceDepth += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
				if current.braceDepth <= 0 {
					length := (i + 1) - current.startLine
					if length > 50 {
						issues = append(issues, AnalysisIssue{
							Line:     current.startLine,
							Message:  fmt.Sprintf("Function is %d lines long (threshold: 50); consider refactoring", length),
							Rule:     "long-function",
							Severity: "medium",
						})
					}
					current = nil
				}
			}

		case "python":
			if (strings.HasPrefix(trimmed, "def ") || strings.HasPrefix(trimmed, "async def ")) && strings.HasSuffix(trimmed, ":") {
				// End the previous function if any.
				if current != nil {
					length := (i + 1) - current.startLine
					if length > 50 {
						issues = append(issues, AnalysisIssue{
							Line:     current.startLine,
							Message:  fmt.Sprintf("Function is %d lines long (threshold: 50); consider refactoring", length),
							Rule:     "long-function",
							Severity: "medium",
						})
					}
				}
				current = &funcInfo{
					name:      trimmed,
					startLine: i + 1,
				}
			}
		}
	}

	// Handle the last function (for Python where functions end implicitly).
	if current != nil && (language == "python") {
		length := len(lines) - current.startLine + 1
		if length > 50 {
			issues = append(issues, AnalysisIssue{
				Line:     current.startLine,
				Message:  fmt.Sprintf("Function is %d lines long (threshold: 50); consider refactoring", length),
				Rule:     "long-function",
				Severity: "medium",
			})
		}
	}

	return issues
}

// detectDeepNesting finds code with more than 4 levels of nesting.
func detectDeepNesting(lines []string, language string) []AnalysisIssue {
	var issues []AnalysisIssue
	const maxDepth = 4

	switch language {
	case "go", "java", "javascript", "typescript", "c", "cpp":
		depth := 0
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			depth += strings.Count(trimmed, "{")
			if depth > maxDepth {
				issues = append(issues, AnalysisIssue{
					Line:     i + 1,
					Message:  fmt.Sprintf("Nesting depth of %d exceeds threshold of %d; consider extracting logic", depth, maxDepth),
					Rule:     "deep-nesting",
					Severity: "medium",
				})
			}
			depth -= strings.Count(trimmed, "}")
			if depth < 0 {
				depth = 0
			}
		}
	case "python":
		for i, line := range lines {
			if len(strings.TrimSpace(line)) == 0 {
				continue
			}
			// Count leading spaces; typical indent is 4 spaces.
			indent := len(line) - len(strings.TrimLeft(line, " \t"))
			tabEquiv := strings.Count(line[:indent], "\t")
			spaceEquiv := indent - tabEquiv
			depth := tabEquiv + spaceEquiv/4
			if depth > maxDepth {
				issues = append(issues, AnalysisIssue{
					Line:     i + 1,
					Message:  fmt.Sprintf("Indentation depth of %d exceeds threshold of %d; consider extracting logic", depth, maxDepth),
					Rule:     "deep-nesting",
					Severity: "medium",
				})
			}
		}
	}

	return issues
}

// detectTodoComments finds TODO, FIXME, HACK comments.
func detectTodoComments(lines []string) []AnalysisIssue {
	var issues []AnalysisIssue
	for i, line := range lines {
		upper := strings.ToUpper(line)
		for _, marker := range []string{"TODO", "FIXME", "HACK", "XXX"} {
			if strings.Contains(upper, marker) {
				// Verify it's in a comment-ish context (contains // or # or /* or *)
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") ||
					strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "/*") {
					issues = append(issues, AnalysisIssue{
						Line:     i + 1,
						Message:  fmt.Sprintf("Found %s comment — ensure this is tracked", marker),
						Rule:     "todo-comment",
						Severity: "low",
					})
					break
				}
			}
		}
	}
	return issues
}

// detectMagicNumbers finds hardcoded numeric literals outside common values (0, 1, 2, etc.).
func detectMagicNumbers(lines []string, language string) []AnalysisIssue {
	var issues []AnalysisIssue
	if language == "go" || language == "java" || language == "javascript" || language == "typescript" {
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			// Skip comments, imports, const/var declarations.
			if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") ||
				strings.HasPrefix(trimmed, "import") || strings.HasPrefix(trimmed, "const") ||
				strings.HasPrefix(trimmed, "var") || strings.HasPrefix(trimmed, "#") {
				continue
			}
			// Look for assignments or comparisons with large numeric literals.
			for _, pat := range magicNumberPatterns {
				if pat.Compiled != nil && pat.Compiled.MatchString(trimmed) {
					issues = append(issues, AnalysisIssue{
						Line:     i + 1,
						Message:  "Consider extracting magic number into a named constant",
						Rule:     "magic-number",
						Severity: "low",
					})
					break
				}
			}
		}
	}
	return issues
}

// detectUncheckedErrors detects Go patterns where errors are discarded.
func detectUncheckedErrors(lines []string, language string) []AnalysisIssue {
	var issues []AnalysisIssue
	if language != "go" {
		return issues
	}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Pattern: function call without capturing the error, e.g., `someFunc()` on its own line
		// More reliable: assignments to `_` for error-returning functions
		if strings.Contains(trimmed, "_ = ") || strings.Contains(trimmed, "_ :=") {
			// Check if the discarded value likely is an error
			if strings.Contains(trimmed, "err") || strings.Contains(trimmed, "Err") || strings.Contains(trimmed, "Error") {
				continue // They're explicitly assigning the error to _, which is at least intentional
			}
		}
		// Detect `result, _ := someFunc()` pattern
		if strings.Contains(trimmed, ", _ :=") || strings.Contains(trimmed, ", _ =") {
			issues = append(issues, AnalysisIssue{
				Line:     i + 1,
				Message:  "Error return value is discarded; consider handling it",
				Rule:     "unchecked-error",
				Severity: "high",
			})
		}
	}
	return issues
}

// detectPotentialNilPointer finds patterns that might cause nil pointer dereferences in Go.
func detectPotentialNilPointer(lines []string, language string) []AnalysisIssue {
	var issues []AnalysisIssue
	if language != "go" {
		return issues
	}
	for i := 0; i < len(lines)-1; i++ {
		trimmed := strings.TrimSpace(lines[i])
		nextTrimmed := ""
		if i+1 < len(lines) {
			nextTrimmed = strings.TrimSpace(lines[i+1])
		}

		// Pattern: assignment from a function call, then using the result without nil check
		// e.g., `val := someMap[key]` followed by `val.Method()`
		if strings.Contains(trimmed, ":=") && strings.Contains(trimmed, "[") && strings.Contains(trimmed, "]") {
			varName := strings.TrimSpace(strings.Split(trimmed, ":=")[0])
			varName = strings.TrimSpace(strings.Split(varName, ",")[0])
			if varName != "" && strings.Contains(nextTrimmed, varName+".") {
				issues = append(issues, AnalysisIssue{
					Line:     i + 2,
					Message:  fmt.Sprintf("Potential nil pointer: %q is used without nil check after map/slice access", varName),
					Rule:     "potential-nil-pointer",
					Severity: "high",
				})
			}
		}
	}
	return issues
}

// detectUnusedParameters looks for function parameters that don't appear in the function body.
func detectUnusedParameters(lines []string, language string) []AnalysisIssue {
	var issues []AnalysisIssue
	if language != "go" {
		return issues
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.Contains(trimmed, "func ") || !strings.Contains(trimmed, "(") {
			continue
		}

		// Extract parameter list between the first set of parentheses after "func".
		funcIdx := strings.Index(trimmed, "func ")
		if funcIdx == -1 {
			continue
		}
		rest := trimmed[funcIdx:]
		// Find parameter parentheses (skip receiver if any).
		parenStart := -1
		parenCount := 0
		firstParenEnd := -1
		for j, ch := range rest {
			if ch == '(' {
				parenCount++
				if parenStart == -1 {
					parenStart = j
				}
			}
			if ch == ')' {
				parenCount--
				if parenCount == 0 && firstParenEnd == -1 {
					firstParenEnd = j
				}
			}
		}

		if parenStart == -1 || firstParenEnd == -1 {
			continue
		}

		// For methods, skip the receiver parens and look at the second pair.
		// This is a simplified check.
		paramStr := rest[parenStart+1 : firstParenEnd]
		if len(paramStr) == 0 {
			continue
		}

		// Check for parameters named _ (already intentionally unused).
		params := strings.Split(paramStr, ",")
		for _, param := range params {
			param = strings.TrimSpace(param)
			parts := strings.Fields(param)
			if len(parts) == 0 || parts[0] == "_" {
				continue
			}
			paramName := parts[0]
			if paramName == "ctx" || paramName == "context" {
				continue // ctx is conventionally always passed
			}

			// Look ahead in the function body for usage.
			braceDepth := 0
			found := false
			started := false
			for j := i; j < len(lines); j++ {
				bl := lines[j]
				braceDepth += strings.Count(bl, "{")
				if braceDepth > 0 {
					started = true
				}
				braceDepth -= strings.Count(bl, "}")
				if started && j > i {
					// Check if parameter name appears in the body.
					if strings.Contains(bl, paramName) {
						found = true
						break
					}
				}
				if started && braceDepth <= 0 {
					break
				}
			}

			if !found && strings.Contains(line, "{") {
				issues = append(issues, AnalysisIssue{
					Line:     i + 1,
					Message:  fmt.Sprintf("Parameter %q may be unused", paramName),
					Rule:     "unused-parameter",
					Severity: "low",
				})
			}
		}
	}

	return issues
}
