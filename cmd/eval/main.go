// Command eval runs the offline review-evaluation harness: it loads the golden PR
// cases from ./evals, runs the review agent over each, scores findings by
// precision/recall/F1, prints a report, and exits non-zero if the overall F1 falls
// below EVAL_MIN_F1 (default 0.5) — suitable as a CI regression gate.
//
// Requires a working LLM configuration (LLM_PROVIDER / LLM_API_KEY / LLM_MODEL).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"

	"github.com/codepilot-ai/codepilot-ai/internal/config"
	"github.com/codepilot-ai/codepilot-ai/internal/eval"
	"github.com/codepilot-ai/codepilot-ai/internal/llm"
)

func main() {
	_ = godotenv.Load()

	dir := flag.String("dir", "evals", "directory of eval case JSON files")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(2)
	}

	cases, err := eval.Load(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load error: %v\n", err)
		os.Exit(2)
	}
	if len(cases) == 0 {
		fmt.Fprintf(os.Stderr, "no eval cases found in %q\n", *dir)
		os.Exit(2)
	}

	client := llm.NewClientWithBaseURL(cfg.LLM.Provider, cfg.LLM.APIKey, cfg.LLM.BaseURL,
		cfg.LLM.Model, cfg.LLM.MaxTokens, cfg.LLM.Temperature)
	reviewer := eval.NewAgentReviewer(client)

	report := eval.Run(context.Background(), cases, reviewer, 0)
	fmt.Print(report.String())

	minF1 := envFloat("EVAL_MIN_F1", 0.5)
	if !report.Pass(minF1) {
		fmt.Fprintf(os.Stderr, "\nFAIL: overall F1 %.2f < threshold %.2f\n", report.Overall.F1, minF1)
		os.Exit(1)
	}
	fmt.Printf("\nPASS: overall F1 %.2f >= threshold %.2f\n", report.Overall.F1, minF1)
}

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}
