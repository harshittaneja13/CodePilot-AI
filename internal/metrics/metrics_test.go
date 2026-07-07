package metrics

import (
	"strings"
	"testing"
)

func TestCounterRender(t *testing.T) {
	r := NewRegistry()
	c := r.NewCounter("reqs_total", "Total requests", "method")
	c.Inc("GET")
	c.Inc("GET")
	c.Add(3, "POST")

	out := r.Render()
	for _, want := range []string{
		"# TYPE reqs_total counter",
		`reqs_total{method="GET"} 2`,
		`reqs_total{method="POST"} 3`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestGaugeRender(t *testing.T) {
	r := NewRegistry()
	g := r.NewGauge("queue_depth", "items queued")
	g.Set(7)
	g.Set(4)
	if out := r.Render(); !strings.Contains(out, "queue_depth 4") {
		t.Errorf("gauge render wrong:\n%s", out)
	}
}

func TestHistogramRender(t *testing.T) {
	r := NewRegistry()
	h := r.NewHistogram("dur_seconds", "duration", []float64{1, 5, 10})
	h.Observe(0.5)
	h.Observe(3)
	h.Observe(20)

	out := r.Render()
	// cumulative: le=1 →1, le=5 →2, le=10 →2, +Inf →3, sum=23.5, count=3
	for _, want := range []string{
		`dur_seconds_bucket{le="1"} 1`,
		`dur_seconds_bucket{le="5"} 2`,
		`dur_seconds_bucket{le="10"} 2`,
		`dur_seconds_bucket{le="+Inf"} 3`,
		"dur_seconds_sum 23.5",
		"dur_seconds_count 3",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestAppHelpersRender(t *testing.T) {
	ReviewCompleted("completed")
	AddReviewTokens(10, 5)
	AddReviewCost(0.0123)
	AddAgentToolCalls(2)
	ObserveReviewDuration(3)
	ObserveHTTP("GET", "/api/health", "200", 0.01)

	out := Render()
	for _, want := range []string{
		"codepilot_reviews_total",
		"codepilot_review_tokens_total",
		"codepilot_http_requests_total",
		"codepilot_review_duration_seconds_count",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in default registry output", want)
		}
	}
}
