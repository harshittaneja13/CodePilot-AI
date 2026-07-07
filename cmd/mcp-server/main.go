// Command mcp-server runs CodePilot as a Model Context Protocol (MCP) server over
// stdio, exposing review history and semantic code retrieval as tools any MCP host can
// call. It shares the app's config, database, and RAG service.
//
// Register with an MCP host (e.g. Claude Desktop) by pointing it at this binary.
// stdout carries the JSON-RPC protocol; all logs go to stderr.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"

	"github.com/codepilot-ai/codepilot-ai/internal/config"
	"github.com/codepilot-ai/codepilot-ai/internal/database"
	"github.com/codepilot-ai/codepilot-ai/internal/mcpserver"
	"github.com/codepilot-ai/codepilot-ai/internal/rag"
	"github.com/codepilot-ai/codepilot-ai/internal/services"
	"github.com/codepilot-ai/codepilot-ai/pkg/logger"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	// CRITICAL: stdout is the MCP JSON-RPC channel, so all logging goes to stderr.
	lvl, perr := zerolog.ParseLevel(cfg.App.LogLevel)
	if perr != nil {
		lvl = zerolog.InfoLevel
	}
	logger.Log = zerolog.New(os.Stderr).Level(lvl).With().Timestamp().Str("service", "codepilot-mcp-server").Logger()
	log := logger.Log

	db, err := database.NewPostgresDB(cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("database connection failed")
	}
	defer db.Close()

	reviewService := services.NewReviewService(db)

	// RAG retrieval tool is registered only when RAG is enabled.
	var retriever mcpserver.ContextRetriever
	if cfg.RAG.Enabled {
		embedder := rag.NewHTTPEmbedder(cfg.RAG.EmbeddingsBaseURL, cfg.RAG.EmbeddingsModel, cfg.RAG.EmbeddingsAPIKey, cfg.RAG.EmbeddingsDim)
		store := rag.NewQdrantStore(cfg.RAG.QdrantURL, cfg.RAG.Collection)
		retriever = rag.NewService(embedder, store, log)
	}

	srv := mcpserver.New("codepilot-ai", "1.0.0", log)
	mcpserver.RegisterCodePilotTools(srv, reviewService, retriever)

	log.Info().Bool("rag", cfg.RAG.Enabled).Msg("CodePilot MCP server ready (stdio)")
	if err := srv.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		log.Error().Err(err).Msg("MCP server stopped")
		os.Exit(1)
	}
}
