# ============================================================================
# CodePilot AI — Backend Dockerfile
# Multi-stage build for a minimal, secure Go binary
# ============================================================================

# ---------------------------------------------------------------------------
# Stage 1: Builder
# We use golang:1.23-alpine for a small image with the Go toolchain.
# ---------------------------------------------------------------------------
FROM golang:1.23-alpine AS builder

# Install git (needed for fetching Go modules from VCS) and
# ca-certificates (needed for HTTPS in the final image).
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy dependency manifests first to leverage Docker layer caching.
# If go.mod/go.sum haven't changed, Docker reuses the cached module download.
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy the rest of the source code.
COPY . .

# Build a statically-linked binary with CGO disabled.
# -ldflags="-s -w" strips debug info to reduce binary size (~30% smaller).
# -trimpath removes local filesystem paths from the binary for reproducibility.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.version=$(date +%Y%m%d)" \
    -trimpath \
    -o /app/codepilot-server \
    ./cmd/server

# ---------------------------------------------------------------------------
# Stage 2: Runtime
# We use alpine:latest for the smallest possible runtime footprint (~5MB).
# ---------------------------------------------------------------------------
FROM alpine:latest

# Labels for container metadata (OCI standard).
LABEL maintainer="CodePilot AI Team"
LABEL org.opencontainers.image.title="codepilot-backend"
LABEL org.opencontainers.image.description="CodePilot AI — MCP-powered GitHub PR Review Agent"
LABEL org.opencontainers.image.source="https://github.com/your-org/codepilot-ai"

# Install ca-certificates, tzdata, and docker-cli.
# docker-cli is required so the backend can spawn the GitHub MCP server
# container via the host Docker socket (mounted at /var/run/docker.sock).
RUN apk add --no-cache ca-certificates tzdata docker-cli

# Create a non-root user for security.
# Running as root inside containers is a well-known anti-pattern.
RUN addgroup -S codepilot && adduser -S codepilot -G codepilot

WORKDIR /app

# Copy the compiled binary from the builder stage.
COPY --from=builder /app/codepilot-server /app/codepilot-server

# Copy database migration files so the server can run migrations on startup.
COPY --from=builder /app/migrations /app/migrations

# Switch to the non-root user.
USER codepilot

# Expose the backend API port.
EXPOSE 8080

# Health check — Docker (and orchestrators like ECS/K8s) will ping this
# endpoint to determine if the container is healthy.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/api/health || exit 1

# Run the server binary.
ENTRYPOINT ["/app/codepilot-server"]
