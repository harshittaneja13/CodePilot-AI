package review

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/codepilot-ai/codepilot-ai/internal/llm"
	"github.com/codepilot-ai/codepilot-ai/internal/models"
)

// runTriage runs Phase 1: the LLM selects which files deserve deep review.
// It returns the files to review (all files if triage is skipped, fails, or selects nothing)
// and the number of tokens the triage call consumed. allFileContexts must be pre-sorted by
// priority so that truncation to maxTriageFiles keeps the most important files.
func (e *Engine) runTriage(ctx context.Context, reviewID string, pr *models.PullRequest, allFileContexts []llm.FileContext, triageModel string, log zerolog.Logger) ([]llm.FileContext, llm.Usage) {
	reviewFileContexts := allFileContexts
	var usage llm.Usage

	if len(allFileContexts) < triageSkipThreshold {
		e.logStep(ctx, reviewID, "triage", "skipped",
			fmt.Sprintf("PR has %d files — below threshold, reviewing all", len(allFileContexts)), 0)
		return reviewFileContexts, usage
	}

	stepStart := time.Now()
	// Cap the file list sent to triage to control token usage.
	triageInputFiles := allFileContexts
	if len(triageInputFiles) > maxTriageFiles {
		triageInputFiles = triageInputFiles[:maxTriageFiles]
	}

	triageMessages := llm.BuildTriagePrompt(pr, triageInputFiles, maxPriorityFiles)
	triageResp, triageErr := e.llmClient.ChatWithModelJSON(ctx, triageMessages, triageModel)
	if triageErr != nil {
		log.Warn().Err(triageErr).Msg("triage call failed; reviewing all files")
		e.logStep(ctx, reviewID, "triage", "failed", triageErr.Error(), time.Since(stepStart).Milliseconds())
		return reviewFileContexts, usage
	}

	usage.Add(triageResp.InputTokens, triageResp.OutputTokens)
	triageResult, parseErr := llm.ParseTriageResponse(triageResp.Content)
	if parseErr != nil {
		log.Warn().Err(parseErr).Msg("triage parse failed; reviewing all files")
		return reviewFileContexts, usage
	}
	if len(triageResult.PriorityFiles) == 0 {
		return reviewFileContexts, usage
	}

	prioritySet := make(map[string]bool, len(triageResult.PriorityFiles))
	for _, p := range triageResult.PriorityFiles {
		prioritySet[p] = true
	}
	var selected []llm.FileContext
	for _, f := range allFileContexts {
		if prioritySet[f.Path] {
			selected = append(selected, f)
		}
	}
	if len(selected) > 0 {
		reviewFileContexts = selected
	}

	e.logStep(ctx, reviewID, "triage", "success",
		fmt.Sprintf("selected %d/%d files: %s | %s",
			len(reviewFileContexts), len(allFileContexts),
			strings.Join(triageResult.PriorityFiles, ", "),
			triageResult.Rationale),
		time.Since(stepStart).Milliseconds())

	return reviewFileContexts, usage
}

// runReview runs Phase 2: the LLM reviews the selected diffs and emits structured findings.
// It returns the parsed result, tokens used, and — on failure — the execution step name and error
// so the caller can mark the review as failed with the correct step.
func (e *Engine) runReview(ctx context.Context, reviewID string, pr *models.PullRequest, reviewFileContexts []llm.FileContext, repoContext, reviewModel string, log zerolog.Logger) (*llm.ReviewResult, llm.Usage, string, error) {
	stepStart := time.Now()

	// analysisResults is reserved for deterministic analyzer output; currently empty.
	messages := llm.BuildReviewPrompt(pr, reviewFileContexts, repoContext, "{}")

	chatResp, err := e.llmClient.ChatWithModelJSON(ctx, messages, reviewModel)
	if err != nil {
		return nil, llm.Usage{}, "llm_review", err
	}
	usage := llm.Usage{InputTokens: chatResp.InputTokens, OutputTokens: chatResp.OutputTokens}
	e.logStep(ctx, reviewID, "llm_review", "success",
		fmt.Sprintf("tokens_used=%d model=%s files_reviewed=%d",
			chatResp.TokensUsed, chatResp.Model, len(reviewFileContexts)),
		time.Since(stepStart).Milliseconds())

	reviewResult, err := llm.ParseReviewResponse(chatResp.Content)
	if err != nil {
		return nil, usage, "parse_response", err
	}

	return reviewResult, usage, "", nil
}

// runReflection runs Phase 3: a second LLM call independently validates each finding and scores
// confidence. Findings marked invalid or below the confidence threshold are filtered out.
// It returns the kept findings, the number filtered, and tokens used. On any failure it keeps all
// findings unchanged.
func (e *Engine) runReflection(ctx context.Context, reviewID string, findings []llm.ReviewFinding, reviewFileContexts []llm.FileContext, reflectionModel string, log zerolog.Logger) ([]llm.ReviewFinding, int, llm.Usage) {
	if len(findings) < 1 {
		return findings, 0, llm.Usage{}
	}

	stepStart := time.Now()
	reflectMessages := llm.BuildReflectionPrompt(findings, reviewFileContexts)
	reflectResp, reflectErr := e.llmClient.ChatWithModelJSON(ctx, reflectMessages, reflectionModel)
	if reflectErr != nil {
		log.Warn().Err(reflectErr).Msg("reflection call failed; keeping all findings")
		e.logStep(ctx, reviewID, "reflection", "failed", reflectErr.Error(), time.Since(stepStart).Milliseconds())
		return findings, 0, llm.Usage{}
	}

	usage := llm.Usage{InputTokens: reflectResp.InputTokens, OutputTokens: reflectResp.OutputTokens}
	valResult, parseErr := llm.ParseReflectionResponse(reflectResp.Content)
	if parseErr != nil {
		log.Warn().Err(parseErr).Msg("reflection parse failed; keeping all findings")
		return findings, 0, usage
	}

	kept, removed := ApplyReflectionValidations(findings, valResult.Validations, confidenceThreshold)
	e.logStep(ctx, reviewID, "reflection", "success",
		fmt.Sprintf("kept=%d filtered=%d tokens=%d", len(kept), len(removed), usage.Total()),
		time.Since(stepStart).Milliseconds())

	return kept, len(removed), usage
}
