// Package review implements the core review processing pipeline.
package review

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/codepilot-ai/codepilot-ai/internal/agent"
	"github.com/codepilot-ai/codepilot-ai/internal/analyzer"
	"github.com/codepilot-ai/codepilot-ai/internal/llm"
	"github.com/codepilot-ai/codepilot-ai/internal/mcp"
	"github.com/codepilot-ai/codepilot-ai/internal/metrics"
	"github.com/codepilot-ai/codepilot-ai/internal/models"
	"github.com/codepilot-ai/codepilot-ai/internal/rag"
)

const (
	// confidenceThreshold is the minimum confidence required to keep a finding.
	// Findings below this threshold are filtered out by the reflection pass.
	confidenceThreshold = 0.7

	// maxPriorityFiles is the maximum number of files the triage LLM can select for deep review.
	maxPriorityFiles = 8

	// triageSkipThreshold — if the PR changes fewer files than this, skip triage and review all.
	triageSkipThreshold = 4

	// maxTriageFiles caps how many files are listed in the triage prompt.
	// Each file line is ~80 tokens; 20 files = ~1600 tokens for the triage call,
	// leaving sufficient headroom for the review + reflection calls within the 12k TPM limit.
	maxTriageFiles = 20
)

// ReviewStore defines the interface for review persistence operations.
type ReviewStore interface {
	Create(ctx context.Context, review *models.Review) (*models.Review, error)
	UpdateStatus(ctx context.Context, id, status, summary, errMsg string) error
	AddComment(ctx context.Context, comment *models.ReviewComment) (*models.ReviewComment, error)
	AddExecutionLog(ctx context.Context, execLog *models.ExecutionLog) error
	UpdateCounts(ctx context.Context, id string, total, critical, high, medium, low, tokensUsed int, processingTimeMs int64) error
	UpdateUsage(ctx context.Context, id string, inputTokens, outputTokens int, costUSD float64) error
}

// PRStore defines the interface for pull request persistence operations.
type PRStore interface {
	Upsert(ctx context.Context, pr *models.PullRequest) (*models.PullRequest, error)
	GetByRepoAndNumber(ctx context.Context, repoID string, number int) (*models.PullRequest, error)
}

// RepoStore defines the interface for repository persistence operations.
type RepoStore interface {
	GetByFullName(ctx context.Context, fullName string) (*models.Repository, error)
}

// ContextBuilderIface defines the interface for building repository context.
type ContextBuilderIface interface {
	BuildContext(ctx context.Context, owner, repo, branch string) (string, error)
}

// Engine orchestrates the full PR review pipeline.
type Engine struct {
	mcpClient      *mcp.Client
	llmClient      *llm.Client
	analyzer       *analyzer.Analyzer
	reviewService  ReviewStore
	prService      PRStore
	repoService    RepoStore
	contextBuilder ContextBuilderIface
	rag            *rag.Service // optional; enables the agent's retrieve_context tool
	// Per-phase model overrides for cost tiering; empty means "fall back" (see resolveModels).
	triageModel     string
	reviewModel     string
	reflectionModel string
	logger          zerolog.Logger
}

// SetRAG attaches a RAG service so the review agent gains semantic code retrieval.
func (e *Engine) SetRAG(svc *rag.Service) { e.rag = svc }

// SetModelTiers configures per-phase models for cost tiering. Any empty value falls
// back to the resolved review model (see resolveModels). Typically triage/reflection
// use a cheap/fast model and review uses a stronger one.
func (e *Engine) SetModelTiers(triage, review, reflection string) {
	e.triageModel = triage
	e.reviewModel = review
	e.reflectionModel = reflection
}

// resolveModels computes the concrete model id for each phase. perRepoModel is the
// per-repository override (may be empty). Review honours the override, then the review
// tier, then the client default; triage/reflection use their tier when set, else review.
func (e *Engine) resolveModels(perRepoModel string) (triage, review, reflection string) {
	review = firstNonEmpty(perRepoModel, e.reviewModel, e.llmClient.GetModel())
	triage = firstNonEmpty(e.triageModel, review)
	reflection = firstNonEmpty(e.reflectionModel, review)
	return triage, review, reflection
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// NewEngine creates a new review Engine with all required dependencies.
func NewEngine(
	mcpClient *mcp.Client,
	llmClient *llm.Client,
	analyzerInst *analyzer.Analyzer,
	reviewService ReviewStore,
	prService PRStore,
	repoService RepoStore,
	contextBuilder ContextBuilderIface,
	logger zerolog.Logger,
) *Engine {
	return &Engine{
		mcpClient:      mcpClient,
		llmClient:      llmClient,
		analyzer:       analyzerInst,
		reviewService:  reviewService,
		prService:      prService,
		repoService:    repoService,
		contextBuilder: contextBuilder,
		logger:         logger.With().Str("component", "review-engine").Logger(),
	}
}

// ProcessPullRequest runs the complete three-phase review pipeline:
//
//	Phase 1 – Triage:    LLM selects which files deserve deep review (skipped for small PRs).
//	Phase 2 – Review:    LLM reviews the selected file diffs and emits findings with confidence scores.
//	Phase 3 – Reflect:   A second LLM call independently validates each finding; low-confidence findings are filtered.
func (e *Engine) ProcessPullRequest(ctx context.Context, owner, repo string, prNumber int, action string) error {
	startTime := time.Now()
	log := e.logger.With().
		Str("owner", owner).
		Str("repo", repo).
		Int("pr_number", prNumber).
		Str("action", action).
		Logger()

	log.Info().Msg("starting PR review pipeline")

	fullName := fmt.Sprintf("%s/%s", owner, repo)

	// ── Step 1: Repository lookup ─────────────────────────────────────────────
	repoModel, err := e.repoService.GetByFullName(ctx, fullName)
	if err != nil {
		log.Error().Err(err).Msg("repository not found in database")
		return fmt.Errorf("repository lookup failed: %w", err)
	}
	if !repoModel.IsActive || (!repoModel.Settings.AutoReview && action != "manual_retry") {
		log.Info().Msg("automatic review disabled for repository")
		return nil
	}

	// ── Step 2: Fetch PR details via MCP ─────────────────────────────────────
	prData, err := e.mcpClient.GetPullRequest(ctx, owner, repo, prNumber)
	if err != nil {
		log.Error().Err(err).Msg("failed to fetch PR from GitHub")
		return fmt.Errorf("fetching PR from GitHub: %w", err)
	}

	// ── Step 3: Upsert the PR in the database ─────────────────────────────────
	pr := mapPRData(prData, repoModel.ID)
	pr, err = e.prService.Upsert(ctx, pr)
	if err != nil {
		log.Error().Err(err).Msg("failed to upsert PR")
		return fmt.Errorf("upserting PR: %w", err)
	}

	// Resolve concrete per-phase models for tiering and cost accounting. reviewModel
	// respects the per-repo override; triage/reflection use their cheaper tier when set.
	triageModel, reviewModel, reflectionModel := e.resolveModels(repoModel.Settings.LLMModel)
	resolvedModel := reviewModel // shown on the review record

	// ── Step 4: Create the review record ──────────────────────────────────────
	review := &models.Review{
		PullRequestID: pr.ID,
		LLMModelStr:   resolvedModel,
	}
	review, err = e.reviewService.Create(ctx, review)
	if err != nil {
		log.Error().Err(err).Msg("failed to create review")
		return fmt.Errorf("creating review record: %w", err)
	}

	reviewID := review.ID
	log = log.With().Str("review_id", reviewID).Logger()

	if err := e.reviewService.UpdateStatus(ctx, reviewID, models.ReviewStatusInProgress, "", ""); err != nil {
		log.Error().Err(err).Msg("failed to update review status")
	}

	// ── Step 5: Fetch changed files ───────────────────────────────────────────
	stepStart := time.Now()
	filesData, err := e.mcpClient.ListPullRequestFiles(ctx, owner, repo, prNumber)
	if err != nil {
		e.failReview(ctx, reviewID, "fetch_files", err, log)
		return fmt.Errorf("fetching PR files: %w", err)
	}
	e.logStep(ctx, reviewID, "fetch_files", "success",
		fmt.Sprintf("fetched %d files", len(filesData)),
		time.Since(stepStart).Milliseconds())

	// Fetch unified diff and split by file (fallback when MCP omits patches).
	diffText, diffErr := e.mcpClient.GetPullRequestDiff(ctx, owner, repo, prNumber)
	if diffErr != nil {
		log.Warn().Err(diffErr).Msg("failed to fetch unified diff")
	}
	diffPatches := splitUnifiedDiff(diffText)

	// ── Step 6: Build repository context ──────────────────────────────────────
	stepStart = time.Now()
	var repoContext string
	if e.contextBuilder != nil {
		repoContext, err = e.contextBuilder.BuildContext(ctx, owner, repo, repoModel.DefaultBranch)
		if err != nil {
			log.Warn().Err(err).Msg("failed to build repository context; continuing without it")
		}
	}

	existingComments, commentsErr := e.mcpClient.GetPullRequestReviewComments(ctx, owner, repo, prNumber)
	if commentsErr != nil {
		log.Warn().Err(commentsErr).Msg("failed to fetch existing review comments")
		existingComments = nil
	}
	e.logStep(ctx, reviewID, "build_context", "success",
		"repository context built",
		time.Since(stepStart).Milliseconds())

	// ── Step 7: Build candidate file list ─────────────────────────────────────
	maxFiles := repoModel.Settings.MaxFilesPerReview
	if maxFiles <= 0 {
		maxFiles = 50
	}

	patches := make(map[string]string)
	var allFileContexts []llm.FileContext

	for _, fileData := range filesData {
		filename, _ := fileData["filename"].(string)
		patch, _ := fileData["patch"].(string)
		if patch == "" {
			patch = diffPatches[filename]
		}
		if filename == "" || patch == "" || isExcluded(filename, repoModel.Settings.ExcludePatterns) {
			continue
		}
		if len(allFileContexts) >= maxFiles {
			break
		}
		if isAutoGenerated(filename) {
			continue
		}
		if len(patch) > 2000 {
			patch = patch[:2000] + "\n... (truncated)"
		}

		adds, _ := fileData["additions"].(float64)
		dels, _ := fileData["deletions"].(float64)

		allFileContexts = append(allFileContexts, llm.FileContext{
			Path:      filename,
			Patch:     patch,
			Language:  detectLanguage(filename),
			Additions: int(adds),
			Deletions: int(dels),
		})
		patches[filename] = patch
	}

	// ── Phase 1: Triage ───────────────────────────────────────────────────────
	// Sort files by priority so that when we must truncate to maxTriageFiles,
	// security-sensitive and high-churn files are always shown to the triage LLM.
	allFileContexts = SortByPriority(allFileContexts)

	var usage llm.Usage
	var costUSD float64

	reviewFileContexts, triageUsage := e.runTriage(ctx, reviewID, pr, allFileContexts, triageModel, log)
	usage.Add(triageUsage.InputTokens, triageUsage.OutputTokens)
	costUSD += llm.CostUSD(triageModel, triageUsage.InputTokens, triageUsage.OutputTokens)

	// ── Phase 2: Review ───────────────────────────────────────────────────────
	// Preferred path: an autonomous agent that decides which tools to call (read a
	// file, search code, read a diff) before emitting findings. Falls back to the
	// deterministic single-shot review when the model lacks tool support or the agent
	// fails for any reason.
	var reviewResult *llm.ReviewResult
	var reviewUsage llm.Usage

	if e.llmClient.SupportsTools() {
		stepStart = time.Now()
		// Index the changed files so the agent's retrieve_context tool can surface
		// cross-file context. Best-effort and skipped entirely when RAG is disabled.
		e.indexForRAG(ctx, owner, repo, fullName, pr.HeadSHAStr, reviewFileContexts, reviewID, log)

		ag := agent.New(e.llmClient, e.mcpClient, log)
		if e.rag != nil {
			ag.SetRetriever(e.rag)
		}
		agRes, agErr := ag.Run(ctx, agent.Input{
			Owner:       owner,
			Repo:        repo,
			HeadRef:     pr.HeadSHAStr,
			PR:          pr,
			Files:       reviewFileContexts,
			RepoContext: repoContext,
			Model:       reviewModel,
		})
		if agErr != nil {
			log.Warn().Err(agErr).Msg("agent review failed; falling back to fixed pipeline")
			e.logStep(ctx, reviewID, "agent_review", "failed", agErr.Error(), time.Since(stepStart).Milliseconds())
		} else {
			reviewResult = &llm.ReviewResult{Summary: agRes.Summary, Findings: agRes.Findings}
			reviewUsage = llm.Usage{InputTokens: agRes.InputTokens, OutputTokens: agRes.OutputTokens}
			metrics.AddAgentToolCalls(len(agRes.Trace))
			e.logStep(ctx, reviewID, "agent_review", "success",
				fmt.Sprintf("steps=%d tool_calls=%d findings=%d tokens=%d",
					agRes.Steps, len(agRes.Trace), len(agRes.Findings), agRes.TokensUsed),
				time.Since(stepStart).Milliseconds())
			// Persist each tool call as an execution-log entry for the dashboard timeline.
			for _, ts := range agRes.Trace {
				e.logStep(ctx, reviewID, "agent_tool", "success",
					fmt.Sprintf("#%d %s(%s) → %s", ts.Step, ts.Tool, ts.Args, ts.Result),
					ts.DurationMs)
			}
		}
	}

	// Fixed-pipeline fallback (also the path for models without tool support).
	if reviewResult == nil {
		rr, ru, failStep, rerr := e.runReview(ctx, reviewID, pr, reviewFileContexts, repoContext, reviewModel, log)
		if rerr != nil {
			e.failReview(ctx, reviewID, failStep, rerr, log)
			if failStep == "parse_response" {
				return fmt.Errorf("parsing LLM response: %w", rerr)
			}
			return fmt.Errorf("LLM review failed: %w", rerr)
		}
		reviewResult = rr
		reviewUsage = ru
	}
	usage.Add(reviewUsage.InputTokens, reviewUsage.OutputTokens)
	costUSD += llm.CostUSD(reviewModel, reviewUsage.InputTokens, reviewUsage.OutputTokens)

	// Apply domain severity rules (security ≥ high, style ≤ medium)
	AdjustSeverities(reviewResult.Findings)
	reviewResult.Findings = DeduplicateFindings(reviewResult.Findings)
	reviewResult.Findings = removePreviouslyPublished(reviewResult.Findings, existingComments)

	// ── Phase 3: Reflection / Self-Critique ───────────────────────────────────
	// A second LLM call independently validates each finding and scores confidence.
	// Findings marked invalid or below the confidence threshold are filtered out.
	keptFindings, filteredCount, reflectUsage := e.runReflection(ctx, reviewID, reviewResult.Findings, reviewFileContexts, reflectionModel, log)
	reviewResult.Findings = keptFindings
	usage.Add(reflectUsage.InputTokens, reflectUsage.OutputTokens)
	costUSD += llm.CostUSD(reflectionModel, reflectUsage.InputTokens, reflectUsage.OutputTokens)
	totalTokens := usage.Total()

	// Append reflection filter note to the review summary so it appears on the dashboard
	if filteredCount > 0 {
		reviewResult.Summary = fmt.Sprintf("%s (%d low-confidence finding(s) filtered by self-critique pass.)",
			reviewResult.Summary, filteredCount)
	}

	// ── Step 13: Publish review to GitHub via MCP ─────────────────────────────
	stepStart = time.Now()
	var mcpComments []mcp.ReviewCommentInput
	publishableFindings := filterPublishableFindings(reviewResult.Findings, patches)
	for _, f := range publishableFindings {
		if f.LineNumber > 0 && f.FilePath != "" {
			body := fmt.Sprintf("**[%s] %s**\n\n%s\n\n**Why it matters:** %s\n\n**Suggestion:** %s",
				strings.ToUpper(f.Severity), f.Title, f.Explanation, f.WhyItMatters, f.Suggestion)
			mcpComments = append(mcpComments, mcp.ReviewCommentInput{
				Path: f.FilePath,
				Line: f.LineNumber,
				Body: body,
			})
		}
	}

	event := "COMMENT"
	if hasCriticalFindings(reviewResult.Findings) {
		event = "REQUEST_CHANGES"
	}

	publishErr := e.mcpClient.CreatePullRequestReview(ctx, owner, repo, prNumber,
		reviewResult.Summary, event, mcpComments)
	if publishErr != nil {
		log.Error().Err(publishErr).Msg("failed to publish review to GitHub; storing locally")
		e.logStep(ctx, reviewID, "publish_review", "failed",
			publishErr.Error(), time.Since(stepStart).Milliseconds())
	} else {
		e.logStep(ctx, reviewID, "publish_review", "success",
			fmt.Sprintf("published %d comments", len(mcpComments)),
			time.Since(stepStart).Milliseconds())
	}

	// ── Step 14: Store comments in the database ───────────────────────────────
	sevCounts := CountBySeverity(reviewResult.Findings)
	for _, f := range reviewResult.Findings {
		comment := &models.ReviewComment{
			ReviewID:        reviewID,
			FilePath:        f.FilePath,
			LineNumber:      sql.NullInt32{Int32: int32(f.LineNumber), Valid: f.LineNumber > 0},
			Severity:        ValidateSeverity(f.Severity),
			Title:           f.Title,
			Explanation:     f.Explanation,
			WhyItMattersStr: f.WhyItMatters,
			SuggestionStr:   f.Suggestion,
			CodeSnippetStr:  f.CodeSnippet,
			Published:       publishErr == nil,
		}
		if _, storeErr := e.reviewService.AddComment(ctx, comment); storeErr != nil {
			log.Error().Err(storeErr).Str("file", f.FilePath).Msg("failed to store comment")
		}
	}

	// ── Step 15: Finalize ─────────────────────────────────────────────────────
	processingTime := time.Since(startTime).Milliseconds()
	if updateErr := e.reviewService.UpdateCounts(ctx, reviewID,
		len(reviewResult.Findings),
		sevCounts[models.SeverityCritical],
		sevCounts[models.SeverityHigh],
		sevCounts[models.SeverityMedium],
		sevCounts[models.SeverityLow],
		totalTokens,
		processingTime,
	); updateErr != nil {
		log.Error().Err(updateErr).Msg("failed to update review counts")
	}

	if updateErr := e.reviewService.UpdateUsage(ctx, reviewID, usage.InputTokens, usage.OutputTokens, costUSD); updateErr != nil {
		log.Error().Err(updateErr).Msg("failed to update review usage/cost")
	}

	if err := e.reviewService.UpdateStatus(ctx, reviewID, models.ReviewStatusCompleted, reviewResult.Summary, ""); err != nil {
		log.Error().Err(err).Msg("failed to complete review status")
	}

	metrics.ReviewCompleted("completed")
	metrics.AddReviewTokens(usage.InputTokens, usage.OutputTokens)
	metrics.AddReviewCost(costUSD)
	metrics.ObserveReviewDuration(float64(processingTime) / 1000.0)

	log.Info().
		Int("findings", len(reviewResult.Findings)).
		Int("filtered_by_reflection", filteredCount).
		Int("total_tokens", totalTokens).
		Int("input_tokens", usage.InputTokens).
		Int("output_tokens", usage.OutputTokens).
		Float64("cost_usd", costUSD).
		Int64("processing_time_ms", processingTime).
		Msg("PR review completed")

	return nil
}

// failReview marks a review as failed and logs the execution step.
func (e *Engine) failReview(ctx context.Context, reviewID, step string, err error, log zerolog.Logger) {
	log.Error().Err(err).Str("step", step).Msg("review pipeline failed")
	metrics.ReviewCompleted("failed")
	_ = e.reviewService.UpdateStatus(ctx, reviewID, models.ReviewStatusFailed, "", err.Error())
	e.logStep(ctx, reviewID, step, "failed", err.Error(), 0)
}

// logStep records an execution log entry.
func (e *Engine) logStep(ctx context.Context, reviewID, step, status, message string, durationMs int64) {
	execLog := &models.ExecutionLog{
		ReviewID:   reviewID,
		Step:       step,
		Status:     status,
		MessageStr: message,
		DurationMs: durationMs,
	}
	if err := e.reviewService.AddExecutionLog(ctx, execLog); err != nil {
		e.logger.Error().Err(err).Str("step", step).Msg("failed to log execution step")
	}
}

// indexForRAG best-effort indexes the changed files' full contents into the vector
// store so the agent's retrieve_context tool has cross-file context to search. It is a
// no-op when RAG is disabled, and never fails the review.
func (e *Engine) indexForRAG(ctx context.Context, owner, repo, fullName, ref string, files []llm.FileContext, reviewID string, log zerolog.Logger) {
	if e.rag == nil {
		return
	}
	start := time.Now()
	var docs []rag.Document
	for _, f := range files {
		content, err := e.mcpClient.GetFileContents(ctx, owner, repo, f.Path, ref)
		if err != nil || strings.TrimSpace(content) == "" {
			continue
		}
		docs = append(docs, rag.Document{Path: f.Path, Language: f.Language, Content: content})
	}
	if len(docs) == 0 {
		return
	}
	n, err := e.rag.Index(ctx, fullName, ref, docs)
	if err != nil {
		log.Warn().Err(err).Msg("rag indexing failed; retrieval may be limited")
		e.logStep(ctx, reviewID, "rag_index", "failed", err.Error(), time.Since(start).Milliseconds())
		return
	}
	e.logStep(ctx, reviewID, "rag_index", "success", fmt.Sprintf("indexed %d files", n), time.Since(start).Milliseconds())
}
