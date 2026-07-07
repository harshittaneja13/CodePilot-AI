package eval

import (
	"context"
	"fmt"
	"strings"

	"github.com/codepilot-ai/codepilot-ai/internal/llm"
)

// Reviewer produces findings for a case. It is implemented by the real agent-backed
// reviewer (see agentReviewer) or by mocks in tests.
type Reviewer interface {
	Review(ctx context.Context, c Case) ([]llm.ReviewFinding, error)
}

// CaseResult is the outcome of evaluating one case.
type CaseResult struct {
	Name     string `json:"name"`
	Score    Score  `json:"score"`
	Produced int    `json:"produced"`
	Expected int    `json:"expected"`
	Err      string `json:"error,omitempty"`
}

// Report aggregates per-case results and a micro-averaged overall score.
type Report struct {
	Cases   []CaseResult `json:"cases"`
	Overall Score        `json:"overall"`
}

// Run evaluates every case with the reviewer and returns a micro-averaged report.
// A reviewer error for a case is recorded (its findings treated as empty) and does not
// abort the whole run.
func Run(ctx context.Context, cases []Case, r Reviewer, tol int) Report {
	if tol <= 0 {
		tol = defaultLineTolerance
	}
	var report Report
	for _, c := range cases {
		produced, err := r.Review(ctx, c)
		cr := CaseResult{Name: c.Name, Expected: len(c.Expected), Produced: len(produced)}
		if err != nil {
			cr.Err = err.Error()
		}
		cr.Score = ScoreFindings(c.Expected, produced, tol)
		report.Overall.add(cr.Score)
		report.Cases = append(report.Cases, cr)
	}
	return report
}

// Pass reports whether the overall F1 meets the minimum threshold.
func (r Report) Pass(minF1 float64) bool { return r.Overall.F1 >= minF1 }

// String renders a human-readable results table.
func (r Report) String() string {
	var sb strings.Builder
	sb.WriteString("Case                                 P      R      F1    (tp/fp/fn)\n")
	sb.WriteString("------------------------------------------------------------------\n")
	for _, c := range r.Cases {
		name := c.Name
		if len(name) > 35 {
			name = name[:35]
		}
		fmt.Fprintf(&sb, "%-35s %.2f   %.2f   %.2f  (%d/%d/%d)",
			name, c.Score.Precision, c.Score.Recall, c.Score.F1, c.Score.TP, c.Score.FP, c.Score.FN)
		if c.Err != "" {
			fmt.Fprintf(&sb, "  ERROR: %s", c.Err)
		}
		sb.WriteString("\n")
	}
	sb.WriteString("------------------------------------------------------------------\n")
	fmt.Fprintf(&sb, "%-35s %.2f   %.2f   %.2f  (%d/%d/%d)\n",
		"OVERALL", r.Overall.Precision, r.Overall.Recall, r.Overall.F1,
		r.Overall.TP, r.Overall.FP, r.Overall.FN)
	return sb.String()
}
