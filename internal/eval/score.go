package eval

import (
	"path/filepath"
	"strings"

	"github.com/codepilot-ai/codepilot-ai/internal/llm"
)

// defaultLineTolerance is how far (in lines) a produced finding may be from the
// expected line and still count as a match — LLMs are often off by a line or two.
const defaultLineTolerance = 3

// Score holds the confusion-matrix counts and derived metrics for a set of findings.
type Score struct {
	TP        int     `json:"tp"`
	FP        int     `json:"fp"`
	FN        int     `json:"fn"`
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
	F1        float64 `json:"f1"`
}

// add merges another score's counts and recomputes metrics (for micro-averaging).
func (s *Score) add(o Score) {
	s.TP += o.TP
	s.FP += o.FP
	s.FN += o.FN
	s.recompute()
}

func (s *Score) recompute() {
	s.Precision = ratio(s.TP, s.TP+s.FP)
	s.Recall = ratio(s.TP, s.TP+s.FN)
	if s.Precision+s.Recall == 0 {
		s.F1 = 0
		return
	}
	s.F1 = 2 * s.Precision * s.Recall / (s.Precision + s.Recall)
}

// ratio returns num/den, defined as 1 when den == 0 (no items → perfect by convention).
func ratio(num, den int) float64 {
	if den == 0 {
		return 1
	}
	return float64(num) / float64(den)
}

// ScoreFindings scores produced findings against expected ones using greedy 1:1
// matching (each expected consumes at most one produced). tol is the line tolerance.
func ScoreFindings(expected []ExpectedFinding, produced []llm.ReviewFinding, tol int) Score {
	usedProduced := make([]bool, len(produced))
	tp := 0

	for _, exp := range expected {
		for i, f := range produced {
			if usedProduced[i] {
				continue
			}
			if findingMatches(exp, f, tol) {
				usedProduced[i] = true
				tp++
				break
			}
		}
	}

	var s Score
	s.TP = tp
	s.FN = len(expected) - tp
	s.FP = len(produced) - tp // produced that matched nothing
	s.recompute()
	return s
}

// findingMatches reports whether a produced finding satisfies an expected one:
// same file, within line tolerance (when a line is specified), an optional severity
// match, and at least one expected keyword present in the title/explanation.
func findingMatches(exp ExpectedFinding, f llm.ReviewFinding, tol int) bool {
	if !sameFile(exp.File, f.FilePath) {
		return false
	}
	if exp.Line > 0 && f.LineNumber > 0 && absInt(exp.Line-f.LineNumber) > tol {
		return false
	}
	if exp.Severity != "" && !strings.EqualFold(exp.Severity, f.Severity) {
		return false
	}
	if len(exp.Keywords) > 0 {
		hay := strings.ToLower(f.Title + " " + f.Explanation)
		if !anyContains(hay, exp.Keywords) {
			return false
		}
	}
	return true
}

func sameFile(a, b string) bool {
	if a == b {
		return true
	}
	// Tolerate leading path differences (e.g. "a/main.go" vs "main.go").
	return filepath.Base(a) == filepath.Base(b) && filepath.Base(a) != ""
}

func anyContains(haystack string, keywords []string) bool {
	for _, k := range keywords {
		if k != "" && strings.Contains(haystack, strings.ToLower(k)) {
			return true
		}
	}
	return false
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
