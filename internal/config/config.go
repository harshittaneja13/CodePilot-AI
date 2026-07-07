// Package config provides configuration management for the CodePilot AI application.
// It reads values from environment variables with sensible defaults and validates
// the resulting configuration.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the application.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	GitHub   GitHubConfig
	LLM      LLMConfig
	RAG      RAGConfig
	App      AppConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host string
	Port int
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	Host         string
	Port         int
	User         string
	Password     string
	DBName       string
	SSLMode      string
	MaxOpenConns int
	MaxIdleConns int
}

// ConnectionString returns a PostgreSQL DSN built from the configuration.
func (d DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode,
	)
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

// Address returns the Redis address in host:port format.
func (r RedisConfig) Address() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

// GitHubConfig holds GitHub-specific settings.
type GitHubConfig struct {
	Token          string
	WebhookSecret  string
	WebhookBaseURL string
	MCPImage       string
	MCPToolsets    string
}

// LLMConfig holds LLM provider settings.
//
// Model is the default/fallback model. The per-phase Triage/Review/Reflection models
// enable cost tiering (a cheap model for triage & reflection, a stronger one for the
// main review). Any left empty falls back to the resolved review model, then Model.
type LLMConfig struct {
	Provider        string
	APIKey          string
	BaseURL         string
	Model           string
	TriageModel     string
	ReviewModel     string
	ReflectionModel string
	MaxTokens       int
	Temperature     float64
}

// RAGConfig holds retrieval-augmented-generation settings (vector DB + embeddings).
// When Enabled is false the review agent runs without the retrieve_context tool.
type RAGConfig struct {
	Enabled           bool
	QdrantURL         string
	Collection        string
	EmbeddingsBaseURL string
	EmbeddingsModel   string
	EmbeddingsAPIKey  string
	EmbeddingsDim     int
}

// AppConfig holds general application settings.
type AppConfig struct {
	Environment string
	LogLevel    string
}

// Load reads configuration from environment variables, applying defaults where needed.
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: getEnvInt("SERVER_PORT", 8080),
		},
		Database: DatabaseConfig{
			Host:         getEnv("DB_HOST", "localhost"),
			Port:         getEnvInt("DB_PORT", 5432),
			User:         getEnv("DB_USER", "codepilot"),
			Password:     getEnv("DB_PASSWORD", "codepilot"),
			DBName:       getEnv("DB_NAME", "codepilot"),
			SSLMode:      getEnv("DB_SSL_MODE", getEnv("DB_SSLMODE", "disable")),
			MaxOpenConns: getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns: getEnvInt("DB_MAX_IDLE_CONNS", 5),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvInt("REDIS_PORT", 6379),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		GitHub: GitHubConfig{
			Token:          getEnv("GITHUB_PERSONAL_ACCESS_TOKEN", ""),
			WebhookSecret:  getEnv("GITHUB_WEBHOOK_SECRET", ""),
			WebhookBaseURL: getEnv("WEBHOOK_BASE_URL", ""),
			MCPImage:       getEnv("GITHUB_MCP_IMAGE", "ghcr.io/github/github-mcp-server"),
			MCPToolsets:    getEnv("GITHUB_MCP_TOOLSETS", "repos,pull_requests,actions"),
		},
		LLM: LLMConfig{
			Provider:        getEnv("LLM_PROVIDER", "openai"),
			APIKey:          getEnv("LLM_API_KEY", ""),
			BaseURL:         getEnv("LLM_BASE_URL", ""),
			Model:           getEnv("LLM_MODEL", "gpt-4"),
			TriageModel:     getEnv("LLM_TRIAGE_MODEL", ""),
			ReviewModel:     getEnv("LLM_REVIEW_MODEL", ""),
			ReflectionModel: getEnv("LLM_REFLECTION_MODEL", ""),
			MaxTokens:       getEnvInt("LLM_MAX_TOKENS", 4096),
			Temperature:     getEnvFloat("LLM_TEMPERATURE", 0.1),
		},
		RAG: RAGConfig{
			Enabled:           getEnvBool("RAG_ENABLED", false),
			QdrantURL:         getEnv("QDRANT_URL", "http://localhost:6333"),
			Collection:        getEnv("QDRANT_COLLECTION", "code_chunks"),
			EmbeddingsBaseURL: getEnv("EMBEDDINGS_BASE_URL", "http://localhost:11434/v1"),
			EmbeddingsModel:   getEnv("EMBEDDINGS_MODEL", "nomic-embed-text"),
			EmbeddingsAPIKey:  getEnv("EMBEDDINGS_API_KEY", ""),
			EmbeddingsDim:     getEnvInt("EMBEDDINGS_DIM", 768),
		},
		App: AppConfig{
			Environment: getEnv("APP_ENVIRONMENT", "development"),
			LogLevel:    getEnv("LOG_LEVEL", getEnv("APP_LOG_LEVEL", "debug")),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// Validate checks the configuration for required values and logical consistency.
func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server port must be between 1 and 65535, got %d", c.Server.Port)
	}

	if c.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}

	if c.Database.DBName == "" {
		return fmt.Errorf("database name is required")
	}

	if c.Database.MaxOpenConns < 1 {
		return fmt.Errorf("database max_open_conns must be at least 1")
	}

	if c.Database.MaxIdleConns < 0 {
		return fmt.Errorf("database max_idle_conns must be non-negative")
	}

	if c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		return fmt.Errorf("database max_idle_conns (%d) cannot exceed max_open_conns (%d)",
			c.Database.MaxIdleConns, c.Database.MaxOpenConns)
	}

	validEnvs := map[string]bool{"development": true, "staging": true, "production": true}
	if !validEnvs[c.App.Environment] {
		return fmt.Errorf("invalid environment '%s': must be development, staging, or production", c.App.Environment)
	}

	validProviders := map[string]bool{"openai": true, "anthropic": true, "compatible": true}
	if !validProviders[c.LLM.Provider] {
		return fmt.Errorf("invalid LLM provider '%s': must be openai, anthropic, or compatible", c.LLM.Provider)
	}

	if c.RAG.Enabled {
		if c.RAG.QdrantURL == "" {
			return fmt.Errorf("QDRANT_URL is required when RAG_ENABLED=true")
		}
		if c.RAG.EmbeddingsBaseURL == "" {
			return fmt.Errorf("EMBEDDINGS_BASE_URL is required when RAG_ENABLED=true")
		}
		if c.RAG.EmbeddingsDim < 1 {
			return fmt.Errorf("EMBEDDINGS_DIM must be a positive integer when RAG_ENABLED=true")
		}
	}

	if c.App.Environment == "production" {
		if c.GitHub.Token == "" {
			return fmt.Errorf("GITHUB_PERSONAL_ACCESS_TOKEN is required in production")
		}
		if c.GitHub.WebhookSecret == "" {
			return fmt.Errorf("GITHUB_WEBHOOK_SECRET is required in production")
		}
		if c.LLM.APIKey == "" {
			return fmt.Errorf("LLM_API_KEY is required in production")
		}
	}

	return nil
}

// Address returns the formatted server address for net/http.ListenAndServe.
func (c *Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// --- Environment variable helpers ---

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	strValue, exists := os.LookupEnv(key)
	if !exists || strValue == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(strValue)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvFloat(key string, defaultValue float64) float64 {
	strValue, exists := os.LookupEnv(key)
	if !exists || strValue == "" {
		return defaultValue
	}
	value, err := strconv.ParseFloat(strValue, 64)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvBool(key string, defaultValue bool) bool {
	strValue, exists := os.LookupEnv(key)
	if !exists || strValue == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(strValue)
	if err != nil {
		return defaultValue
	}
	return value
}
