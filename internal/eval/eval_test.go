package eval

import (
	"context"
	"math"
	"testing"

	"github.com/codepilot-ai/codepilot-ai/internal/llm"
)

func finding(file string, line int, title, expl, sev string) llm.ReviewFinding {
	return llm.ReviewFinding{FilePath: file, LineNumber: line, Title: title, Explanation: expl, Severity: sev}
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestScoreFindingsPerfect(t *testing.T) {
	expected := []ExpectedFinding{{File: "main.go", Line: 10, Keywords: []string{"nil"}}}
	produced := []llm.ReviewFinding{finding("main.go", 10, "nil deref", "x may be nil", "high")}
	s := ScoreFindings(expected, produced, defaultLineTolerance)
	if s.TP != 1 || s.FP != 0 || s.FN != 0 {
		t.Fatalf("counts = %+v, want tp1 fp0 fn0", s)
	}
	if !approx(s.Precision, 1) || !approx(s.Recall, 1) || !approx(s.F1, 1) {
		t.Errorf("metrics = %+v, want all 1.0", s)
	}
}

func TestScoreFindingsPartialWithFalsePositive(t *testing.T) {
	expected := []ExpectedFinding{
		{File: "a.go", Line: 5, Keywords: []string{"leak"}},
		{File: "b.go", Line: 9, Keywords: []string{"race"}},
	}
	produced := []llm.ReviewFinding{
		finding("a.go", 5, "resource leak", "file not closed", "high"), // matches expected[0]
		finding("c.go", 1, "style nit", "rename this", "low"),          // false positive
	}
	s := ScoreFindings(expected, produced, defaultLineTolerance)
	if s.TP != 1 || s.FP != 1 || s.FN != 1 {
		t.Fatalf("counts = %+v, want tp1 fp1 fn1", s)
	}
	if !approx(s.Precision, 0.5) || !approx(s.Recall, 0.5) || !approx(s.F1, 0.5) {
		t.Errorf("metrics = %+v, want 0.5 each", s)
	}
}

func TestScoreFindingsLineTolerance(t *testing.T) {
	expected := []ExpectedFinding{{File: "main.go", Line: 40, Keywords: []string{"overflow"}}}
	produced := []llm.ReviewFinding{finding("main.go", 42, "integer overflow", "may overflow", "medium")}
	if s := ScoreFindings(expected, produced, 3); s.TP != 1 {
		t.Errorf("within tolerance should match, got %+v", s)
	}
	if s := ScoreFindings(expected, produced, 1); s.TP != 0 {
		t.Errorf("outside tolerance should not match, got %+v", s)
	}
}

func TestScoreFindingsEmptyAndFalsePositives(t *testing.T) {
	// Nothing expected, nothing produced → perfect by convention.
	if s := ScoreFindings(nil, nil, defaultLineTolerance); !approx(s.F1, 1) {
		t.Errorf("empty/empty F1 = %f, want 1", s.F1)
	}
	// Nothing expected but findings produced → all false positives.
	produced := []llm.ReviewFinding{finding("a.go", 1, "x", "y", "low"), finding("b.go", 2, "x", "y", "low")}
	s := ScoreFindings(nil, produced, defaultLineTolerance)
	if s.FP != 2 || !approx(s.Precision, 0) || !approx(s.F1, 0) {
		t.Errorf("all-FP score = %+v", s)
	}
}

func TestSameFileBaseMatch(t *testing.T) {
	if !sameFile("a/main.go", "main.go") {
		t.Error("expected base-name file match")
	}
	if sameFile("x/util.go", "y/other.go") {
		t.Error("unexpected file match")
	}
}

func TestLoadRealFixtures(t *testing.T) {
	// The golden set lives at repo-root /evals; tests run from internal/eval.
	cases, err := Load("../../evals")
	if err != nil {
		t.Fatalf("Load evals: %v", err)
	}
	if len(cases) < 2 {
		t.Fatalf("expected at least 2 golden cases, got %d", len(cases))
	}
	for _, c := range cases {
		if c.Name == "" || len(c.PR.Files) == 0 || len(c.Expected) == 0 {
			t.Errorf("case %q is malformed: %+v", c.Name, c)
		}
		for _, e := range c.Expected {
			if e.File == "" || len(e.Keywords) == 0 {
				t.Errorf("case %q has an expected finding without file/keywords: %+v", c.Name, e)
			}
		}
	}
}

// mockReviewer returns preset findings per case name.
type mockReviewer struct{ byCase map[string][]llm.ReviewFinding }

func (m mockReviewer) Review(_ context.Context, c Case) ([]llm.ReviewFinding, error) {
	return m.byCase[c.Name], nil
}

func TestRunAndThreshold(t *testing.T) {
	cases := []Case{
		{Name: "c1", Expected: []ExpectedFinding{{File: "a.go", Line: 1, Keywords: []string{"bug"}}}},
		{Name: "c2", Expected: []ExpectedFinding{{File: "b.go", Line: 1, Keywords: []string{"leak"}}}},
	}
	rev := mockReviewer{byCase: map[string][]llm.ReviewFinding{
		"c1": {finding("a.go", 1, "a bug", "boom", "high")}, // match
		"c2": {},                                            // miss
	}}

	report := Run(context.Background(), cases, rev, defaultLineTolerance)
	// Overall: tp1 fp0 fn1 → precision 1, recall 0.5, F1 = 2*1*0.5/1.5 = 0.6667
	if report.Overall.TP != 1 || report.Overall.FN != 1 {
		t.Fatalf("overall counts = %+v", report.Overall)
	}
	if !approx(report.Overall.Recall, 0.5) {
		t.Errorf("recall = %f, want 0.5", report.Overall.Recall)
	}
	if !report.Pass(0.6) {
		t.Errorf("expected pass at 0.6, F1=%f", report.Overall.F1)
	}
	if report.Pass(0.9) {
		t.Errorf("expected fail at 0.9, F1=%f", report.Overall.F1)
	}
}
