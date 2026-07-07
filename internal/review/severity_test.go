package review

import (
	"testing"

	"github.com/codepilot-ai/codepilot-ai/internal/llm"
)

// ── FilterByConfidence ────────────────────────────────────────────────────────

func TestFilterByConfidence_RemovesLowConfidence(t *testing.T) {
	findings := []llm.ReviewFinding{
		{Title: "real issue", Confidence: 0.9},
		{Title: "uncertain", Confidence: 0.5},
		{Title: "false positive", Confidence: 0.2},
	}
	kept, removed := FilterByConfidence(findings, 0.7)
	if len(kept) != 1 {
		t.Errorf("expected 1 kept finding, got %d", len(kept))
	}
	if kept[0].Title != "real issue" {
		t.Errorf("expected 'real issue' to be kept, got %q", kept[0].Title)
	}
	if len(removed) != 2 {
		t.Errorf("expected 2 removed findings, got %d", len(removed))
	}
}

func TestFilterByConfidence_KeepsZeroConfidence(t *testing.T) {
	// Confidence == 0 means the LLM did not emit the field; keep the finding.
	findings := []llm.ReviewFinding{
		{Title: "no confidence set", Confidence: 0},
	}
	kept, removed := FilterByConfidence(findings, 0.7)
	if len(kept) != 1 {
		t.Errorf("expected 1 kept finding, got %d", len(kept))
	}
	if len(removed) != 0 {
		t.Errorf("expected 0 removed findings, got %d", len(removed))
	}
}

func TestFilterByConfidence_AllAboveThreshold(t *testing.T) {
	findings := []llm.ReviewFinding{
		{Title: "a", Confidence: 0.8},
		{Title: "b", Confidence: 0.95},
		{Title: "c", Confidence: 1.0},
	}
	kept, removed := FilterByConfidence(findings, 0.7)
	if len(kept) != 3 {
		t.Errorf("expected all 3 kept, got %d", len(kept))
	}
	if len(removed) != 0 {
		t.Errorf("expected 0 removed, got %d", len(removed))
	}
}

func TestFilterByConfidence_Empty(t *testing.T) {
	kept, removed := FilterByConfidence(nil, 0.7)
	if len(kept) != 0 || len(removed) != 0 {
		t.Error("empty input should produce empty output")
	}
}

// ── ApplyReflectionValidations ────────────────────────────────────────────────

func TestApplyReflectionValidations_FiltersInvalidFindings(t *testing.T) {
	findings := []llm.ReviewFinding{
		{Title: "real bug", Confidence: 0.9},
		{Title: "nitpick", Confidence: 0.8},
	}
	validations := []llm.FindingValidation{
		{Index: 0, IsValid: true, Confidence: 0.95, Reason: "genuine SQL risk"},
		{Index: 1, IsValid: false, Confidence: 0.15, Reason: "style preference"},
	}
	kept, removed := ApplyReflectionValidations(findings, validations, 0.7)
	if len(kept) != 1 {
		t.Errorf("expected 1 kept finding, got %d", len(kept))
	}
	if kept[0].Title != "real bug" {
		t.Errorf("expected 'real bug' to be kept, got %q", kept[0].Title)
	}
	if len(removed) != 1 || removed[0].Title != "nitpick" {
		t.Errorf("expected 'nitpick' to be removed, got: %v", removed)
	}
}

func TestApplyReflectionValidations_UpdatesConfidenceFromReflection(t *testing.T) {
	findings := []llm.ReviewFinding{
		{Title: "finding", Confidence: 0.8},
	}
	// Reflection downgrades confidence below threshold
	validations := []llm.FindingValidation{
		{Index: 0, IsValid: true, Confidence: 0.3, Reason: "actually uncertain"},
	}
	kept, removed := ApplyReflectionValidations(findings, validations, 0.7)
	if len(kept) != 0 {
		t.Errorf("expected 0 kept (confidence downgraded to 0.3), got %d", len(kept))
	}
	if len(removed) != 1 {
		t.Errorf("expected 1 removed, got %d", len(removed))
	}
}

func TestApplyReflectionValidations_KeepsFindingsWithNoValidation(t *testing.T) {
	findings := []llm.ReviewFinding{
		{Title: "orphan", Confidence: 0.9},
	}
	// No validations returned at all (reflection returned partial results)
	kept, removed := ApplyReflectionValidations(findings, nil, 0.7)
	if len(kept) != 1 {
		t.Errorf("expected 1 kept finding (no validation returned), got %d", len(kept))
	}
	if len(removed) != 0 {
		t.Errorf("expected 0 removed, got %d", len(removed))
	}
}

// ── ValidateSeverity ──────────────────────────────────────────────────────────

func TestValidateSeverity_KnownValues(t *testing.T) {
	cases := map[string]string{
		"critical": "critical",
		"high":     "high",
		"medium":   "medium",
		"low":      "low",
		"CRITICAL": "critical", // case-insensitive
		"HIGH":     "high",
	}
	for input, want := range cases {
		if got := ValidateSeverity(input); got != want {
			t.Errorf("ValidateSeverity(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestValidateSeverity_UnknownDefaultsMedium(t *testing.T) {
	for _, input := range []string{"", "unknown", "info", "warning"} {
		if got := ValidateSeverity(input); got != "medium" {
			t.Errorf("ValidateSeverity(%q) = %q, want 'medium'", input, got)
		}
	}
}

// ── AdjustSeverities ──────────────────────────────────────────────────────────

func TestAdjustSeverities_SecurityUpgradedToHigh(t *testing.T) {
	findings := []llm.ReviewFinding{
		{Title: "SQL injection risk", Explanation: "user input in query", Severity: "low"},
	}
	AdjustSeverities(findings)
	if findings[0].Severity != "high" {
		t.Errorf("security finding should be upgraded to at least 'high', got %q", findings[0].Severity)
	}
}

func TestAdjustSeverities_StyleCappedAtMedium(t *testing.T) {
	findings := []llm.ReviewFinding{
		{Title: "Poor naming convention", Explanation: "style issue with spacing", Severity: "critical"},
	}
	AdjustSeverities(findings)
	if findings[0].Severity != "medium" {
		t.Errorf("style finding should be capped at 'medium', got %q", findings[0].Severity)
	}
}
