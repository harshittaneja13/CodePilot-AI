package llm

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/codepilot-ai/codepilot-ai/internal/models"
)

// FileContext provides context about a file for the LLM review prompt.
type FileContext struct {
	Path           string `json:"path"`
	Patch          string `json:"patch"`
	CurrentContent string `json:"current_content,omitempty"`
	Language       string `json:"language"`
	Additions      int    `json:"additions"`
	Deletions      int    `json:"deletions"`
}

// ── Token budget ──────────────────────────────────────────────────────────────
// Source code is ~2.5 chars/token (denser than prose). With a 12k TPM free-tier
// limit on llama-3.3-70b-versatile, we allocate budgets across all three calls:
//   Phase 1 triage  ≈  400 tokens  (file list, no diffs)
//   Phase 2 review  ≈ 8 000 tokens (selected diffs + system + metadata)
//   Phase 3 reflect ≈ 1 000 tokens (findings JSON + system)
// Total ≈ 9 400 tokens — safely under 12k TPM.
const (
	maxTotalChars = 16000 // diff chars for phase 2 review (≈ 6 400 tokens)
	maxPatchChars = 2000  // per-file diff ceiling
)

// ── Phase 1: Triage ───────────────────────────────────────────────────────────

// BuildTriagePrompt builds the messages for the triage call.
// The LLM receives the file list (no diffs) and decides which files are most
// security/logic-critical and deserve deep review in phase 2.
func BuildTriagePrompt(pr *models.PullRequest, files []FileContext, maxPriority int) []ChatMessage {
	systemPrompt := fmt.Sprintf(`You are a code review triage agent. Given a pull request's changed file list, select which files most need careful review.

Output ONLY valid JSON — no markdown, no explanation outside the JSON object:
{"priority_files": ["path/to/file.go", "path/to/other.ts"], "rationale": "One concise sentence."}

Rules:
- Select at most %d files
- Prioritize: authentication, authorization, database queries, API handlers, input validation, payment logic, security-sensitive code
- Deprioritize: test helpers, documentation, config with trivial value changes, generated files`, maxPriority)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("PR: %s\n", pr.Title))

	if pr.BodyStr != "" {
		desc := pr.BodyStr
		if len(desc) > 400 {
			desc = desc[:400] + "..."
		}
		sb.WriteString("\n<pr_description>\n")
		sb.WriteString(desc)
		sb.WriteString("\n</pr_description>\n")
	}

	sb.WriteString("\nChanged files:\n")
	for _, f := range files {
		sb.WriteString(fmt.Sprintf("- %s (%s) +%d -%d\n", f.Path, f.Language, f.Additions, f.Deletions))
	}
	sb.WriteString("\nSelect the files that most need careful code review.")

	return []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: sb.String()},
	}
}

// ParseTriageResponse extracts the TriageResult from the triage LLM response.
func ParseTriageResponse(content string) (*TriageResult, error) {
	content = extractJSON(content)
	var result TriageResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("parsing triage response: %w", err)
	}
	return &result, nil
}

// ── Phase 2: Review ───────────────────────────────────────────────────────────

// BuildReviewPrompt constructs the messages for the main LLM code review call.
// User-provided content (PR description, file diffs) is wrapped in XML delimiters
// to mitigate prompt injection from malicious PR content.
func BuildReviewPrompt(pr *models.PullRequest, files []FileContext, repoContext string, analysisResults string) []ChatMessage {
	systemPrompt := `You are an expert code reviewer. Analyse the pull request diff and output ONLY a valid JSON object — no markdown fences, no extra text.

Required format:
{
  "summary": "2-3 sentence overview of PR quality and key concerns",
  "findings": [
    {
      "file_path": "path/to/file",
      "line_number": 42,
      "title": "Short issue title",
      "explanation": "What is wrong and why",
      "why_it_matters": "Impact on security, reliability, performance or maintainability",
      "suggestion": "Concrete fix or approach",
      "severity": "critical|high|medium|low",
      "code_snippet": "The problematic code (verbatim from diff)",
      "confidence": 0.95
    }
  ]
}

Severity guide: critical=security/data-loss/crash, high=logic error/resource leak/missing error handling, medium=performance/validation/code smell, low=readability/naming.
Confidence guide: 1.0=certain genuine issue, 0.8=very likely, 0.6=probable, 0.4=uncertain, 0.2=likely false positive.
Only reference line numbers present in the diff. Report only genuine issues. Set confidence honestly.`

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Pull Request: %s\n\n", pr.Title))
	sb.WriteString(fmt.Sprintf("**Author:** %s | **Branch:** %s → %s | **Changes:** +%d/-%d across %d files\n\n",
		pr.Author, pr.HeadBranchStr, pr.BaseBranchStr, pr.Additions, pr.Deletions, pr.ChangedFiles))

	if pr.BodyStr != "" {
		body := pr.BodyStr
		if len(body) > 800 {
			body = body[:800] + "..."
		}
		sb.WriteString("<pr_description>\n")
		sb.WriteString(body)
		sb.WriteString("\n</pr_description>\n\n")
	}

	if repoContext != "" {
		ctx := repoContext
		if len(ctx) > 800 {
			ctx = ctx[:800] + "\n... (truncated)"
		}
		sb.WriteString("<repository_context>\n")
		sb.WriteString(ctx)
		sb.WriteString("\n</repository_context>\n\n")
	}

	if analysisResults != "" && analysisResults != "{}" {
		sb.WriteString("<static_analysis>\n")
		sb.WriteString(analysisResults)
		sb.WriteString("\n</static_analysis>\n\n")
	}

	sb.WriteString("### Changed Files\n\n")
	remainingChars := maxTotalChars
	for _, f := range files {
		if remainingChars <= 0 {
			sb.WriteString("(Additional files omitted — context budget reached.)\n")
			break
		}
		sb.WriteString(fmt.Sprintf("#### %s (%s)\n<diff>\n", f.Path, f.Language))
		patch := f.Patch
		if len(patch) > maxPatchChars {
			patch = patch[:maxPatchChars] + "\n... (truncated)"
		}
		if len(patch) > remainingChars {
			patch = patch[:remainingChars] + "\n... (truncated)"
		}
		sb.WriteString(patch)
		sb.WriteString("\n</diff>\n\n")
		remainingChars -= len(patch)
	}

	return []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: sb.String()},
	}
}

// ParseReviewResponse extracts the ReviewResult JSON from the LLM response.
func ParseReviewResponse(content string) (*ReviewResult, error) {
	content = extractJSON(content)
	var result ReviewResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response as JSON: %w\nContent: %.500s", err, content)
	}
	if result.Findings == nil {
		result.Findings = []ReviewFinding{}
	}
	return &result, nil
}

// ── Phase 3: Reflection / Self-Critique ──────────────────────────────────────

// BuildReflectionPrompt builds the messages for the reflection (self-critique) call.
// The LLM receives each finding from phase 2 and independently validates whether
// it is a genuine actionable issue, returning a confidence score for each.
// Diffs are NOT resent — only a compact file summary and the findings JSON.
func BuildReflectionPrompt(findings []ReviewFinding, files []FileContext) []ChatMessage {
	systemPrompt := `You are a senior code reviewer validating AI-generated findings for accuracy and actionability.

Output ONLY valid JSON — no markdown, no text outside the JSON:
{"validations": [{"index": 0, "is_valid": true, "confidence": 0.9, "reason": "Genuine SQL injection risk — user input concatenated into query"}]}

Confidence guide: 1.0=definitely real, 0.8=very likely, 0.6=uncertain, 0.4=likely false positive, 0.0=definitely wrong.
Mark is_valid=false and confidence<0.5 for findings that:
- Are subjective style preferences with no correctness impact
- Are vague or non-actionable ("consider refactoring")
- Reference a file or concept not in the changed file list
- Are exact duplicates of another finding in the list`

	var sb strings.Builder

	sb.WriteString("## Changed File Summary\n")
	for _, f := range files {
		sb.WriteString(fmt.Sprintf("- %s (%s): +%d -%d lines\n", f.Path, f.Language, f.Additions, f.Deletions))
	}

	sb.WriteString("\n## Findings to Validate\n")
	// Compact representation: only the fields needed for validation
	type compactFinding struct {
		Index       int    `json:"index"`
		FilePath    string `json:"file_path"`
		LineNumber  int    `json:"line_number"`
		Title       string `json:"title"`
		Explanation string `json:"explanation"`
		Severity    string `json:"severity"`
		CodeSnippet string `json:"code_snippet"`
	}
	compact := make([]compactFinding, len(findings))
	for i, f := range findings {
		compact[i] = compactFinding{
			Index:       i,
			FilePath:    f.FilePath,
			LineNumber:  f.LineNumber,
			Title:       f.Title,
			Explanation: f.Explanation,
			Severity:    f.Severity,
			CodeSnippet: f.CodeSnippet,
		}
	}
	findingsJSON, _ := json.MarshalIndent(compact, "", "  ")
	sb.Write(findingsJSON)
	sb.WriteString("\n\nValidate every finding above. Return one validation entry per finding (same indices).")

	return []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: sb.String()},
	}
}

// ParseReflectionResponse extracts the ValidationResult from the reflection LLM response.
func ParseReflectionResponse(content string) (*ValidationResult, error) {
	content = extractJSON(content)
	var result ValidationResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("parsing reflection response: %w", err)
	}
	return &result, nil
}

// ── Shared helpers ────────────────────────────────────────────────────────────

// extractJSON strips markdown fences and extracts the outermost JSON object from content.
func extractJSON(content string) string {
	content = strings.TrimSpace(content)

	// Strip markdown code block fences if present
	if strings.HasPrefix(content, "```") {
		lines := strings.Split(content, "\n")
		startIdx, endIdx := 0, len(lines)
		for i, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				startIdx = i + 1
				break
			}
		}
		for i := len(lines) - 1; i > startIdx; i-- {
			if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
				endIdx = i
				break
			}
		}
		if startIdx < endIdx {
			content = strings.Join(lines[startIdx:endIdx], "\n")
		}
	}

	content = strings.TrimSpace(content)

	// Find outermost JSON object boundaries
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")
	if jsonStart >= 0 && jsonEnd > jsonStart {
		content = content[jsonStart : jsonEnd+1]
	}

	return content
}
