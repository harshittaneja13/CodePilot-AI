package llm

import (
	"math"
	"testing"
)

func almostEqual(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestCostUSD(t *testing.T) {
	tests := []struct {
		name        string
		model       string
		in, out     int
		wantUSD     float64
	}{
		{"gpt-4o 1M+1M", "gpt-4o", 1_000_000, 1_000_000, 2.50 + 10.00},
		{"gpt-4o-mini specific over gpt-4o", "gpt-4o-mini", 1_000_000, 0, 0.15},
		{"gpt-4 variant substring", "gpt-4o-2024-08-06", 2_000_000, 0, 5.00},
		{"claude sonnet", "claude-3-5-sonnet-latest", 1_000_000, 1_000_000, 3.00 + 15.00},
		{"groq llama is free", "llama-3.3-70b-versatile", 5_000_000, 5_000_000, 0},
		{"unknown model is free", "some-random-model", 1_000_000, 1_000_000, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CostUSD(tt.model, tt.in, tt.out)
			if !almostEqual(got, tt.wantUSD) {
				t.Errorf("CostUSD(%q, %d, %d) = %.6f, want %.6f", tt.model, tt.in, tt.out, got, tt.wantUSD)
			}
		})
	}
}

func TestUsageAccumulation(t *testing.T) {
	var u Usage
	u.Add(100, 50)
	u.Add(10, 5)
	if u.InputTokens != 110 || u.OutputTokens != 55 {
		t.Errorf("usage = %+v, want {110, 55}", u)
	}
	if u.Total() != 165 {
		t.Errorf("total = %d, want 165", u.Total())
	}
}
