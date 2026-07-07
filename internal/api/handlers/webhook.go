package handlers

import (
	"context"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	webhookprocessor "github.com/codepilot-ai/codepilot-ai/internal/webhook"
)

// WebhookHandler handles incoming GitHub webhook requests.
type WebhookHandler struct {
	WebhookSecret string
	ProcessFunc   func(ctx context.Context, eventType, deliveryID string, payload []byte) error
	logger        zerolog.Logger
}

// NewWebhookHandler creates a new WebhookHandler.
func NewWebhookHandler(secret string, processFunc func(ctx context.Context, eventType, deliveryID string, payload []byte) error, logger zerolog.Logger) *WebhookHandler {
	return &WebhookHandler{
		WebhookSecret: secret,
		ProcessFunc:   processFunc,
		logger:        logger.With().Str("component", "webhook-handler").Logger(),
	}
}

// HandleWebhook verifies the GitHub webhook signature, reads the payload,
// and spawns a goroutine to process the event asynchronously.
func (h *WebhookHandler) HandleWebhook(c *gin.Context) {
	signature := c.GetHeader("X-Hub-Signature-256")

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to read webhook body")
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "failed to read request body",
		})
		return
	}

	// Verify signature if a secret is configured
	if h.WebhookSecret != "" {
		if !webhookprocessor.VerifySignature(body, signature, h.WebhookSecret) {
			h.logger.Warn().Str("signature", signature).Msg("invalid webhook signature")
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    http.StatusUnauthorized,
				"message": "invalid signature",
			})
			return
		}
	}

	eventType := c.GetHeader("X-GitHub-Event")
	deliveryID := c.GetHeader("X-GitHub-Delivery")

	h.logger.Info().
		Str("event_type", eventType).
		Str("delivery_id", deliveryID).
		Int("payload_size", len(body)).
		Msg("webhook received")

	// Processing only validates and durably enqueues the event, so it is safe to
	// complete before acknowledging the delivery. This prevents accepted events
	// from being lost if the process exits immediately after responding.
	if h.ProcessFunc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"message": "webhook processing unavailable"})
		return
	}
	if err := h.ProcessFunc(c.Request.Context(), eventType, deliveryID, body); err != nil {
		h.logger.Error().Err(err).Str("event_type", eventType).Str("delivery_id", deliveryID).Msg("webhook enqueue failed")
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to enqueue webhook"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message":     "webhook accepted",
		"delivery_id": deliveryID,
	})
}
