// Package webhook handles parsing and processing of incoming GitHub webhook events.
package webhook

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog"
)

// ProcessFunc is a function that processes a PR event with the given owner, repo, number, and action.
type ProcessFunc func(ctx context.Context, deliveryID, owner, repo string, prNumber int, action string) error

// Handler processes incoming GitHub webhook events.
type Handler struct {
	processFunc ProcessFunc
	logger      zerolog.Logger
}

// NewHandler creates a new webhook Handler.
func NewHandler(processFunc ProcessFunc, logger zerolog.Logger) *Handler {
	return &Handler{
		processFunc: processFunc,
		logger:      logger.With().Str("component", "webhook-processor").Logger(),
	}
}

// pullRequestEvent represents the structure of a GitHub pull_request webhook event payload.
type pullRequestEvent struct {
	Action string `json:"action"`
	Number int    `json:"number"`
	PullRequest struct {
		Draft bool `json:"draft"`
		User  struct {
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"user"`
	} `json:"pull_request"`
	Repository struct {
		FullName string `json:"full_name"`
		Owner    struct {
			Login string `json:"login"`
		} `json:"owner"`
		Name string `json:"name"`
	} `json:"repository"`
}

// ProcessWebhook parses the webhook payload based on event type and dispatches processing.
func (h *Handler) ProcessWebhook(ctx context.Context, eventType, deliveryID string, payload []byte) error {
	switch eventType {
	case "pull_request":
		return h.processPullRequestEvent(ctx, deliveryID, payload)
	default:
		h.logger.Debug().Str("event_type", eventType).Msg("ignoring unhandled event type")
		return nil
	}
}

// processPullRequestEvent handles pull_request webhook events.
func (h *Handler) processPullRequestEvent(ctx context.Context, deliveryID string, payload []byte) error {
	var event pullRequestEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("parsing pull_request event: %w", err)
	}

	log := h.logger.With().
		Str("action", event.Action).
		Int("pr_number", event.Number).
		Str("repo", event.Repository.FullName).
		Logger()

	// Only process relevant actions
	validActions := map[string]bool{
		"opened":      true,
		"synchronize": true,
		"reopened":    true,
	}

	if !validActions[event.Action] {
		log.Debug().Msg("ignoring pull_request action")
		return nil
	}

	// Ignore draft PRs
	if event.PullRequest.Draft {
		log.Debug().Msg("ignoring draft PR")
		return nil
	}

	// Ignore bot authors
	if event.PullRequest.User.Type == "Bot" {
		log.Debug().Str("author", event.PullRequest.User.Login).Msg("ignoring bot-authored PR")
		return nil
	}

	owner := event.Repository.Owner.Login
	repo := event.Repository.Name

	log.Info().
		Str("owner", owner).
		Str("repo", repo).
		Msg("processing pull request event")

	if err := h.processFunc(ctx, deliveryID, owner, repo, event.Number, event.Action); err != nil {
		return fmt.Errorf("processing PR #%d: %w", event.Number, err)
	}

	return nil
}
