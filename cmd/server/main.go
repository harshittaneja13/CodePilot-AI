package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/codepilot-ai/codepilot-ai/internal/analyzer"
	"github.com/codepilot-ai/codepilot-ai/internal/api"
	"github.com/codepilot-ai/codepilot-ai/internal/api/handlers"
	"github.com/codepilot-ai/codepilot-ai/internal/config"
	"github.com/codepilot-ai/codepilot-ai/internal/database"
	"github.com/codepilot-ai/codepilot-ai/internal/jobs"
	"github.com/codepilot-ai/codepilot-ai/internal/llm"
	"github.com/codepilot-ai/codepilot-ai/internal/mcp"
	"github.com/codepilot-ai/codepilot-ai/internal/models"
	"github.com/codepilot-ai/codepilot-ai/internal/rag"
	"github.com/codepilot-ai/codepilot-ai/internal/repository"
	"github.com/codepilot-ai/codepilot-ai/internal/review"
	"github.com/codepilot-ai/codepilot-ai/internal/services"
	"github.com/codepilot-ai/codepilot-ai/internal/webhook"
	"github.com/codepilot-ai/codepilot-ai/pkg/logger"
)

func main() {
	// Load .env file if present (non-fatal if missing)
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	logger.Init(cfg.App.Environment, cfg.App.LogLevel)
	log := logger.Log

	log.Info().
		Str("env", cfg.App.Environment).
		Str("version", "1.0.0").
		Msg("starting CodePilot AI")

	// ── Database ──────────────────────────────────────────────────────────────

	db, err := database.NewPostgresDB(cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("database connection failed")
	}
	defer db.Close()

	if err := database.RunMigrations(db); err != nil {
		log.Fatal().Err(err).Msg("database migrations failed")
	}

	// ── Services ──────────────────────────────────────────────────────────────

	repoService := services.NewRepositoryService(db)
	prService := services.NewPullRequestService(db)
	reviewService := services.NewReviewService(db)
	dashboardService := services.NewDashboardService(db)

	// ── MCP client ────────────────────────────────────────────────────────────

	mcpClient := mcp.NewClient(
		cfg.GitHub.Token,
		cfg.GitHub.MCPImage,
		cfg.GitHub.MCPToolsets,
		log,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.GitHub.Token != "" {
		if err := mcpClient.Connect(ctx); err != nil {
			log.Warn().Err(err).Msg("MCP server unavailable; PR review functionality will be degraded")
		} else {
			defer mcpClient.Close()
		}
	} else {
		log.Warn().Msg("GITHUB_PERSONAL_ACCESS_TOKEN not set; MCP features disabled")
	}

	// ── LLM client ────────────────────────────────────────────────────────────

	llmClient := llm.NewClientWithBaseURL(
		cfg.LLM.Provider,
		cfg.LLM.APIKey,
		cfg.LLM.BaseURL,
		cfg.LLM.Model,
		cfg.LLM.MaxTokens,
		cfg.LLM.Temperature,
	)

	// ── Review pipeline ───────────────────────────────────────────────────────

	contextBuilder := repository.NewContextBuilder(mcpClient)
	analyzerInst := analyzer.NewAnalyzer(log)

	engine := review.NewEngine(
		mcpClient,
		llmClient,
		analyzerInst,
		reviewService,
		prService,
		repoService,
		contextBuilder,
		log,
	)

	// Per-phase model tiering (cheap model for triage/reflection, stronger for review).
	engine.SetModelTiers(cfg.LLM.TriageModel, cfg.LLM.ReviewModel, cfg.LLM.ReflectionModel)

	// ── RAG (optional) ────────────────────────────────────────────────────────
	// When enabled, the review agent gains a retrieve_context tool backed by Qdrant
	// + a local embeddings sidecar. Disabled by default; failures are non-fatal.
	if cfg.RAG.Enabled {
		embedder := rag.NewHTTPEmbedder(cfg.RAG.EmbeddingsBaseURL, cfg.RAG.EmbeddingsModel, cfg.RAG.EmbeddingsAPIKey, cfg.RAG.EmbeddingsDim)
		store := rag.NewQdrantStore(cfg.RAG.QdrantURL, cfg.RAG.Collection)
		ragSvc := rag.NewService(embedder, store, log)
		if err := ragSvc.EnsureReady(ctx); err != nil {
			log.Warn().Err(err).Msg("RAG initialization failed; continuing without semantic retrieval")
		} else {
			engine.SetRAG(ragSvc)
			log.Info().Str("qdrant", cfg.RAG.QdrantURL).Str("embeddings_model", cfg.RAG.EmbeddingsModel).Msg("RAG enabled")
		}
	}

	// ── Job queue ─────────────────────────────────────────────────────────────

	queue := jobs.NewQueue(db, engine.ProcessPullRequest, 2, log)
	if err := queue.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("failed to start job queue")
	}

	// ── Webhook processor ─────────────────────────────────────────────────────

	webhookProcessor := webhook.NewHandler(
		func(ctx context.Context, deliveryID, owner, repo string, prNumber int, action string) error {
			return queue.Enqueue(ctx, deliveryID, owner, repo, prNumber, action)
		},
		log,
	)

	// ── Handler closures ──────────────────────────────────────────────────────

	// syncPRs is forward-declared so connectRepo can reference it in its goroutine.
	var syncPRs func(context.Context, string) (int, error)

	// connectRepo fetches repository metadata from GitHub via MCP and persists it.
	connectRepo := func(ctx context.Context, owner, name string) (*models.Repository, error) {
		repoData, err := mcpClient.GetRepository(ctx, owner, name)
		if err != nil {
			return nil, fmt.Errorf("fetching repository %s/%s from GitHub: %w", owner, name, err)
		}

		req := &models.CreateRepositoryRequest{
			Owner:         owner,
			Name:          name,
			FullName:      owner + "/" + name,
			DefaultBranch: "main",
		}

		if id, ok := repoData["id"].(float64); ok {
			req.GitHubID = int64(id)
		}
		if fullName, ok := repoData["full_name"].(string); ok {
			req.FullName = fullName
		}
		if desc, ok := repoData["description"].(string); ok {
			req.Description = desc
		}
		if branch, ok := repoData["default_branch"].(string); ok {
			req.DefaultBranch = branch
		}
		if lang, ok := repoData["language"].(string); ok {
			req.Language = lang
		}

		repo, createErr := repoService.Create(ctx, req)
		if createErr != nil {
			return nil, createErr
		}

		go func() {
			bgCtx := context.Background()

			// Register a webhook on GitHub so new PRs sync automatically.
			if cfg.GitHub.WebhookBaseURL != "" {
				webhookURL := cfg.GitHub.WebhookBaseURL + "/api/webhooks/github"
				if hookID, hookErr := mcpClient.CreateWebhook(bgCtx, owner, name, webhookURL, cfg.GitHub.WebhookSecret); hookErr != nil {
					log.Warn().Err(hookErr).Str("repo", repo.FullName).Msg("auto-register webhook failed")
				} else {
					_ = repoService.UpdateWebhookID(bgCtx, repo.ID, hookID)
					log.Info().Int64("webhook_id", hookID).Str("repo", repo.FullName).Msg("webhook registered on GitHub")
				}
			}

			// Sync existing PRs so the UI shows them immediately.
			if n, syncErr := syncPRs(bgCtx, repo.ID); syncErr != nil {
				log.Warn().Err(syncErr).Str("repo", repo.FullName).Msg("auto-sync PRs failed")
			} else {
				log.Info().Int("count", n).Str("repo", repo.FullName).Msg("auto-synced PRs on connect")
			}
		}()

		return repo, nil
	}

	// syncPRs fetches pull requests from GitHub and upserts them for a given repo UUID.
	syncPRs = func(ctx context.Context, repoID string) (int, error) {
		repo, err := repoService.GetByID(ctx, repoID)
		if err != nil {
			return 0, fmt.Errorf("repo not found: %w", err)
		}
		rawPRs, err := mcpClient.SyncPullRequests(ctx, repo.Owner, repo.Name)
		if err != nil {
			return 0, fmt.Errorf("fetching PRs from GitHub: %w", err)
		}
		count := 0
		for _, raw := range rawPRs {
			pr := &models.PullRequest{RepositoryID: repoID}
			if num, ok := raw["number"].(float64); ok {
				pr.GitHubNumber = int(num)
			}
			if t, ok := raw["title"].(string); ok {
				pr.Title = t
			}
			if b, ok := raw["body"].(string); ok {
				pr.BodyStr = b
			}
			if s, ok := raw["state"].(string); ok {
				pr.State = s
			}
			if user, ok := raw["user"].(map[string]interface{}); ok {
				if login, ok := user["login"].(string); ok {
					pr.Author = login
				}
			}
			if head, ok := raw["head"].(map[string]interface{}); ok {
				if ref, ok := head["ref"].(string); ok {
					pr.HeadBranchStr = ref
				}
				if sha, ok := head["sha"].(string); ok {
					pr.HeadSHAStr = sha
				}
			}
			if base, ok := raw["base"].(map[string]interface{}); ok {
				if ref, ok := base["ref"].(string); ok {
					pr.BaseBranchStr = ref
				}
			}
			if u, ok := raw["html_url"].(string); ok {
				pr.GitHubURLStr = u
			}
			if pr.GitHubNumber == 0 || pr.Title == "" {
				continue
			}
			if _, err := prService.Upsert(ctx, pr); err != nil {
				log.Warn().Err(err).Int("pr", pr.GitHubNumber).Msg("failed to upsert PR")
				continue
			}
			count++
		}
		return count, nil
	}

	// retryReview looks up the review, its PR, and the repository, then enqueues a manual retry job.
	retryReview := func(reviewID string) error {
		bgCtx := context.Background()

		rwc, err := reviewService.GetByID(bgCtx, reviewID)
		if err != nil {
			return fmt.Errorf("looking up review %s: %w", reviewID, err)
		}

		pr, err := prService.GetByID(bgCtx, rwc.Review.PullRequestID)
		if err != nil {
			return fmt.Errorf("looking up pull request %s: %w", rwc.Review.PullRequestID, err)
		}

		repo, err := repoService.GetByID(bgCtx, pr.RepositoryID)
		if err != nil {
			return fmt.Errorf("looking up repository %s: %w", pr.RepositoryID, err)
		}

		return queue.EnqueueManual(bgCtx, repo.Owner, repo.Name, pr.GitHubNumber)
	}

	// ── HTTP handlers ─────────────────────────────────────────────────────────

	// enrichStats fetches diff stats from GitHub for a PR and persists them.
	enrichStats := func(ctx context.Context, prID string) error {
		pr, err := prService.GetByID(ctx, prID)
		if err != nil {
			return err
		}
		repo, err := repoService.GetByID(ctx, pr.RepositoryID)
		if err != nil {
			return err
		}
		additions, deletions, changedFiles, err := mcpClient.FetchPRStats(ctx, repo.Owner, repo.Name, pr.GitHubNumber)
		if err != nil {
			return err
		}
		return prService.UpdateStats(ctx, prID, additions, deletions, changedFiles)
	}

	// fetchFiles fetches changed files from GitHub for a given PR UUID.
	fetchFiles := func(ctx context.Context, prID string) ([]map[string]interface{}, error) {
		pr, err := prService.GetByID(ctx, prID)
		if err != nil {
			return nil, err
		}
		repo, err := repoService.GetByID(ctx, pr.RepositoryID)
		if err != nil {
			return nil, err
		}
		return mcpClient.FetchPRFiles(ctx, repo.Owner, repo.Name, pr.GitHubNumber)
	}

	// triggerReview enqueues an AI review job for a PR given its UUID.
	// If model is non-empty it is saved as the repo's preferred model before enqueuing.
	triggerReview := func(ctx context.Context, prID, model string) error {
		pr, err := prService.GetByID(ctx, prID)
		if err != nil {
			return err
		}
		repo, err := repoService.GetByID(ctx, pr.RepositoryID)
		if err != nil {
			return err
		}
		if model != "" && model != repo.Settings.LLMModel {
			_ = repoService.UpdateSettings(ctx, repo.ID, &models.UpdateRepositorySettingsRequest{LLMModel: &model})
		}
		return queue.EnqueueManual(ctx, repo.Owner, repo.Name, pr.GitHubNumber)
	}

	modelsHandler := handlers.NewModelsHandler(llmClient.ListModels)
	dashHandler := handlers.NewDashboardHandler(dashboardService)
	repoHandler := handlers.NewRepositoryHandler(repoService, connectRepo, syncPRs)
	prHandler := handlers.NewPullRequestHandler(prService, reviewService, enrichStats, fetchFiles, triggerReview)
	reviewHandler := handlers.NewReviewHandler(reviewService, retryReview)
	webhookHTTPHandler := handlers.NewWebhookHandler(
		cfg.GitHub.WebhookSecret,
		webhookProcessor.ProcessWebhook,
		log,
	)

	router := api.NewRouter(api.RouterDeps{
		DashboardHandler:   dashHandler,
		RepositoryHandler:  repoHandler,
		PullRequestHandler: prHandler,
		ReviewHandler:      reviewHandler,
		WebhookHandler:     webhookHTTPHandler,
		ModelsHandler:      modelsHandler,
	}, log)

	// ── HTTP server with graceful shutdown ────────────────────────────────────

	server := &http.Server{
		Addr:         cfg.Address(),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info().Str("addr", cfg.Address()).Msg("HTTP server listening")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server error")
		}
	}()

	<-quit
	log.Info().Msg("shutdown signal received")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("HTTP server shutdown error")
	}

	queue.Wait()
	log.Info().Msg("shutdown complete")
}
