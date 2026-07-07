package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ModelsHandler serves the list of available LLM models.
type ModelsHandler struct {
	listFn func(ctx context.Context) ([]string, error)
}

func NewModelsHandler(listFn func(ctx context.Context) ([]string, error)) *ModelsHandler {
	return &ModelsHandler{listFn: listFn}
}

// blockedModels excludes non-chat models and small/fast models unsuitable for code review
// (either due to quality or Groq free-tier TPM limits too low to handle review prompts).
var blockedModels = map[string]bool{
	// Non-chat / audio / safety models
	"whisper-large-v3": true, "whisper-large-v3-turbo": true,
	"meta-llama/llama-prompt-guard-2-22m": true, "meta-llama/llama-prompt-guard-2-86m": true,
	"openai/gpt-oss-safeguard-20b": true,
	"canopylabs/orpheus-v1-english": true, "canopylabs/orpheus-arabic-saudi": true,
	// Models with <10 000 TPM free-tier limit — too tight for the 3-call pipeline.
	// Triage (~3k) + review (~7k) + reflection (~2k) can exceed 10k tokens per minute.
	// Verified limits via x-ratelimit-limit-tokens header (2026-07-07):
	"llama-3.1-8b-instant":  true, // 6 000 TPM
	"llama3-8b-8192":        true, // 6 000 TPM
	"gemma-7b-it":           true, // 6 000 TPM
	"gemma2-9b-it":          true, // 6 000 TPM
	"qwen/qwen3-32b":        true, // 6 000 TPM
	"allam-2-7b":            true, // 6 000 TPM
	"openai/gpt-oss-120b":   true, // 8 000 TPM
	"openai/gpt-oss-20b":    true, // 8 000 TPM
	"qwen/qwen3.6-27b":      true, // 8 000 TPM
}

func (h *ModelsHandler) List(c *gin.Context) {
	models, err := h.listFn(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, []string{})
		return
	}
	chat := make([]string, 0, len(models))
	for _, m := range models {
		if !blockedModels[m] {
			chat = append(chat, m)
		}
	}
	c.JSON(http.StatusOK, chat)
}
