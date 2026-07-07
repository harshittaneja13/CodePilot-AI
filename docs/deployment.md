# 🚀 CodePilot AI — Production Deployment Guide

> A comprehensive guide to deploying CodePilot AI in a production environment.

---

## Table of Contents

- [Prerequisites](#prerequisites)
- [Environment Setup](#environment-setup)
- [GitHub Webhook Configuration](#github-webhook-configuration)
- [Docker Deployment](#docker-deployment)
- [Manual Deployment](#manual-deployment)
- [SSL/TLS with Nginx Reverse Proxy](#ssltls-with-nginx-reverse-proxy)
- [Monitoring & Health Checks](#monitoring--health-checks)
- [Scaling](#scaling)
- [Backup Strategy](#backup-strategy)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

Ensure all of the following are installed and accessible before proceeding.

| Requirement | Minimum Version | Check Command | Notes |
|---|---|---|---|
| **Docker** | 20.10+ | `docker --version` | Required for container-based deployment |
| **Docker Compose** | 2.0+ | `docker compose version` | V2 plugin format preferred |
| **Go** | 1.23+ | `go version` | Only for manual (non-Docker) deployment |
| **Node.js** | 20+ | `node --version` | Only for manual frontend builds |
| **npm** | 10+ | `npm --version` | Ships with Node.js 20+ |
| **PostgreSQL** | 16+ | `psql --version` | Docker image used: `postgres:16-alpine` |
| **Redis** | 7+ | `redis-server --version` | Docker image used: `redis:7-alpine` |
| **Git** | 2.30+ | `git --version` | For cloning the repository |

### GitHub Requirements

| Requirement | How to Obtain |
|---|---|
| **GitHub Personal Access Token (PAT)** | [Create one](https://github.com/settings/tokens) with `repo` scope (full control of private repos) |
| **GitHub Webhook Secret** | A random string you generate (e.g., `openssl rand -hex 32`) |

### LLM Provider Requirements

| Provider | API Key Source |
|---|---|
| **OpenAI** | [platform.openai.com/api-keys](https://platform.openai.com/api-keys) |
| **Anthropic** | [console.anthropic.com](https://console.anthropic.com/) |

---

## Environment Setup

CodePilot AI uses environment variables for all configuration. A template is provided at `.env.example`.

### Step 1: Create the Environment File

```bash
cp .env.example .env
chmod 600 .env  # Restrict permissions — this file contains secrets
```

### Step 2: Configure All Variables

Edit `.env` with your production values:

```bash
# ============================================================================
# Server
# ============================================================================
SERVER_HOST=0.0.0.0          # Bind address (0.0.0.0 for all interfaces)
SERVER_PORT=8080             # Internal API port
APP_ENVIRONMENT=production   # Must be: development | staging | production
LOG_LEVEL=info               # Log verbosity: debug | info | warn | error

# ============================================================================
# Database (PostgreSQL)
# ============================================================================
DB_HOST=postgres             # Service name in Docker Compose (or hostname)
DB_PORT=5432                 # PostgreSQL port
DB_USER=codepilot            # Database user
DB_PASSWORD=<STRONG_PASSWORD> # ⚠️ Use a strong, unique password
DB_NAME=codepilot            # Database name
DB_SSL_MODE=disable          # Use 'require' for remote/cloud databases
DB_MAX_OPEN_CONNS=25         # Max open database connections (pool size)
DB_MAX_IDLE_CONNS=5          # Max idle connections kept alive

# ============================================================================
# Redis
# ============================================================================
REDIS_HOST=redis             # Service name in Docker Compose (or hostname)
REDIS_PORT=6379              # Redis port
REDIS_PASSWORD=              # Set if Redis requires authentication
REDIS_DB=0                   # Redis database number

# ============================================================================
# GitHub
# ============================================================================
GITHUB_PERSONAL_ACCESS_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx  # PAT with repo scope
GITHUB_WEBHOOK_SECRET=<RANDOM_SECRET>                  # Must match webhook config
GITHUB_MCP_IMAGE=ghcr.io/github/github-mcp-server     # MCP server Docker image

# ============================================================================
# LLM Provider
# ============================================================================
LLM_PROVIDER=openai          # openai | anthropic
LLM_API_KEY=sk-xxxxxxxx     # API key for your LLM provider
LLM_MODEL=gpt-4o            # Model identifier
LLM_MAX_TOKENS=4096         # Maximum response tokens
LLM_TEMPERATURE=0.3         # Lower = more deterministic (0.0–1.0)
```

### Environment Variable Reference

| Variable | Required | Default | Description |
|---|:---:|---|---|
| `SERVER_HOST` | No | `0.0.0.0` | Network interface to bind to |
| `SERVER_PORT` | No | `8080` | Port for the Go API server |
| `APP_ENVIRONMENT` | No | `development` | Controls logging format and behavior |
| `LOG_LEVEL` | No | `debug` | Minimum log level |
| `DB_HOST` | Yes | `postgres` | PostgreSQL hostname |
| `DB_PORT` | No | `5432` | PostgreSQL port |
| `DB_USER` | Yes | `codepilot` | Database username |
| `DB_PASSWORD` | **Yes** | — | Database password |
| `DB_NAME` | Yes | `codepilot` | Database name |
| `DB_SSL_MODE` | No | `disable` | `disable`, `require`, `verify-ca`, `verify-full` |
| `DB_MAX_OPEN_CONNS` | No | `25` | Connection pool: max open connections |
| `DB_MAX_IDLE_CONNS` | No | `5` | Connection pool: max idle connections |
| `REDIS_HOST` | Yes | `redis` | Redis hostname |
| `REDIS_PORT` | No | `6379` | Redis port |
| `REDIS_PASSWORD` | No | — | Redis authentication password |
| `REDIS_DB` | No | `0` | Redis database index |
| `GITHUB_PERSONAL_ACCESS_TOKEN` | **Yes** | — | GitHub PAT with `repo` scope |
| `GITHUB_WEBHOOK_SECRET` | **Yes** | — | HMAC secret for webhook signature verification |
| `GITHUB_MCP_IMAGE` | No | `ghcr.io/github/github-mcp-server` | Docker image for the GitHub MCP server |
| `LLM_PROVIDER` | Yes | `openai` | LLM provider (`openai` or `anthropic`) |
| `LLM_API_KEY` | **Yes** | — | API key for the LLM provider |
| `LLM_MODEL` | No | `gpt-4o` | LLM model identifier |
| `LLM_MAX_TOKENS` | No | `4096` | Max tokens per LLM response |
| `LLM_TEMPERATURE` | No | `0.3` | Sampling temperature for LLM responses |

> [!CAUTION]
> **Never** commit `.env` to version control. The `.gitignore` already excludes it, but always verify.

---

## GitHub Webhook Configuration

GitHub webhooks enable CodePilot AI to receive real-time notifications when PRs are opened or updated.

### Step-by-Step Setup

#### 1. Navigate to Webhook Settings

Go to your GitHub repository → **Settings** → **Webhooks** → **Add webhook**.

> If you're configuring this for an organization, go to the organization's **Settings** → **Webhooks**.

#### 2. Configure the Webhook

Fill in the following fields:

| Field | Value |
|---|---|
| **Payload URL** | `https://your-domain.com/api/webhook` |
| **Content type** | `application/json` |
| **Secret** | The same value as your `GITHUB_WEBHOOK_SECRET` environment variable |

> [!IMPORTANT]
> The Payload URL **must** be publicly accessible from GitHub's servers. For local development, use a tunnel like [ngrok](https://ngrok.com) or [Cloudflare Tunnel](https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/).

#### 3. Select Events

Under **"Which events would you like to trigger this webhook?"**, select:

- ○ **Let me select individual events**
  - ✅ **Pull requests** — Triggers on PR open, close, reopen, edit, synchronize (new push)

Uncheck all other events to minimize noise.

#### 4. Enable the Webhook

- ✅ **Active** — Ensure the webhook is enabled
- Click **Add webhook**

#### 5. Verify the Webhook

After creating the webhook, GitHub sends a `ping` event. Verify it reached your server:

```bash
# Check backend logs for the ping event
docker compose logs backend | grep -i "ping\|webhook"
```

You should see a log entry confirming receipt of the ping event.

#### Local Development Tunneling

For local development without a public URL:

```bash
# Option 1: ngrok
ngrok http 8080
# Copy the HTTPS URL and use it as the Payload URL:
# https://<random-id>.ngrok.io/api/webhook

# Option 2: Cloudflare Tunnel
cloudflared tunnel --url http://localhost:8080
```

---

## Docker Deployment

Docker Compose is the recommended deployment method. It orchestrates all four services: PostgreSQL, Redis, Go backend, and React frontend.

### Quick Deploy

```bash
# 1. Clone the repository
git clone https://github.com/your-org/codepilot-ai.git
cd codepilot-ai

# 2. Configure environment
cp .env.example .env
# Edit .env with production values (see Environment Setup above)

# 3. Build and start all services
docker compose build
docker compose up -d

# 4. Verify all services are running
docker compose ps
```

### Service Architecture

| Service | Container Name | Image | Internal Port | External Port |
|---|---|---|---|---|
| PostgreSQL | `codepilot-postgres` | `postgres:16-alpine` | 5432 | 5432 |
| Redis | `codepilot-redis` | `redis:7-alpine` | 6379 | 6379 |
| Backend | `codepilot-backend` | Custom (multi-stage Go build) | 8080 | 8080 |
| Frontend | `codepilot-frontend` | Custom (Vite build + Nginx) | 80 | 3000 |

### Verify Deployment

```bash
# Health check
curl -s http://localhost:8080/api/health | jq
# Expected output:
# {
#   "status": "healthy",
#   "version": "20260705",
#   "uptime": "2m30s",
#   "database": "connected",
#   "redis": "connected"
# }

# Check all services
docker compose ps

# View logs
docker compose logs -f backend    # Follow backend logs
docker compose logs -f postgres   # Follow database logs
docker compose logs --tail=50     # Last 50 lines from all services
```

### Docker Compose Commands Reference

```bash
docker compose up -d              # Start all services (detached)
docker compose down               # Stop and remove containers
docker compose down -v            # Stop + remove volumes (⚠️ destroys data)
docker compose restart backend    # Restart a specific service
docker compose build --no-cache   # Rebuild images from scratch
docker compose exec backend sh    # Shell into backend container
docker compose exec postgres psql -U codepilot codepilot  # Connect to DB
```

### Production Docker Overrides

For production, consider creating a `docker-compose.prod.yml` override:

```yaml
# docker-compose.prod.yml
services:
  postgres:
    ports: []  # Don't expose DB port externally
    environment:
      POSTGRES_PASSWORD: ${DB_PASSWORD}  # Use strong secret

  redis:
    ports: []  # Don't expose Redis externally
    command: >
      redis-server
      --appendonly yes
      --maxmemory 512mb
      --maxmemory-policy allkeys-lru
      --requirepass ${REDIS_PASSWORD}

  backend:
    restart: always
    deploy:
      resources:
        limits:
          cpus: '2.0'
          memory: 1G
        reservations:
          cpus: '0.5'
          memory: 256M
    logging:
      driver: json-file
      options:
        max-size: "50m"
        max-file: "5"

  frontend:
    restart: always
    deploy:
      resources:
        limits:
          cpus: '0.5'
          memory: 256M
```

Deploy with:

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

> [!NOTE]
> The backend container mounts the Docker socket (`/var/run/docker.sock`) as read-only so it can spawn GitHub MCP server containers on demand. This is required for MCP functionality.

---

## Manual Deployment

For environments where Docker is not available, or for fine-grained control.

### 1. Database Setup

```bash
# Install PostgreSQL 16
# macOS:
brew install postgresql@16

# Linux (Debian/Ubuntu):
sudo apt-get install postgresql-16

# Start the service
sudo systemctl enable --now postgresql

# Create user and database
sudo -u postgres createuser --createdb codepilot
sudo -u postgres psql -c "ALTER USER codepilot WITH PASSWORD '<STRONG_PASSWORD>';"
sudo -u postgres createdb -O codepilot codepilot
```

### 2. Redis Setup

```bash
# macOS:
brew install redis
brew services start redis

# Linux (Debian/Ubuntu):
sudo apt-get install redis-server
sudo systemctl enable --now redis-server
```

### 3. Backend Build & Run

```bash
# Install Go 1.23+ from https://go.dev/dl/

# Clone the repo
git clone https://github.com/your-org/codepilot-ai.git
cd codepilot-ai

# Download dependencies
go mod download

# Build the binary
CGO_ENABLED=0 go build \
  -ldflags="-s -w" \
  -trimpath \
  -o bin/codepilot-server \
  ./cmd/server

# Run migrations (ensure DB env vars are set)
make migrate-up

# Set environment variables
export DB_HOST=localhost
export REDIS_HOST=localhost
# ... set all required variables from .env.example

# Start the server
./bin/codepilot-server
```

### 4. Frontend Build & Serve

```bash
cd frontend

# Install dependencies
npm ci

# Build for production
npm run build

# Serve with a static file server (or configure Nginx)
npx serve dist -l 3000
```

### 5. Running as a Systemd Service

Create `/etc/systemd/system/codepilot.service`:

```ini
[Unit]
Description=CodePilot AI Backend
After=network.target postgresql.service redis.service
Requires=postgresql.service redis.service

[Service]
Type=simple
User=codepilot
Group=codepilot
WorkingDirectory=/opt/codepilot-ai
EnvironmentFile=/opt/codepilot-ai/.env
ExecStart=/opt/codepilot-ai/bin/codepilot-server
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/codepilot-ai

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now codepilot
sudo journalctl -u codepilot -f  # View logs
```

---

## SSL/TLS with Nginx Reverse Proxy

For production deployments, terminate SSL/TLS at Nginx and proxy to the backend and frontend.

### Nginx Configuration

Create `/etc/nginx/sites-available/codepilot`:

```nginx
# Redirect HTTP → HTTPS
server {
    listen 80;
    server_name codepilot.your-domain.com;
    return 301 https://$host$request_uri;
}

# HTTPS server
server {
    listen 443 ssl http2;
    server_name codepilot.your-domain.com;

    # --- SSL Certificates (Let's Encrypt / Certbot) ---
    ssl_certificate     /etc/letsencrypt/live/codepilot.your-domain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/codepilot.your-domain.com/privkey.pem;

    # --- SSL Hardening ---
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384;
    ssl_prefer_server_ciphers off;
    ssl_session_timeout 1d;
    ssl_session_cache shared:SSL:10m;
    ssl_session_tickets off;

    # --- Security Headers ---
    add_header Strict-Transport-Security "max-age=63072000; includeSubDomains" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-Frame-Options "DENY" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;

    # --- API Backend (Go) ---
    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Webhook payloads can be large
        client_max_body_size 10m;

        # Timeouts for long-running review operations
        proxy_read_timeout 120s;
        proxy_send_timeout 120s;
    }

    # --- Frontend (React via Vite/Nginx) ---
    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # --- Static Asset Caching ---
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        proxy_pass http://127.0.0.1:3000;
        expires 30d;
        add_header Cache-Control "public, immutable";
    }
}
```

### Obtain SSL Certificates with Certbot

```bash
# Install Certbot
sudo apt-get install certbot python3-certbot-nginx

# Obtain certificate (interactive)
sudo certbot --nginx -d codepilot.your-domain.com

# Auto-renewal is set up by Certbot. Verify:
sudo certbot renew --dry-run
```

### Enable and Test

```bash
sudo ln -s /etc/nginx/sites-available/codepilot /etc/nginx/sites-enabled/
sudo nginx -t          # Test configuration
sudo systemctl reload nginx
```

---

## Monitoring & Health Checks

### Built-in Health Check

The backend exposes a health endpoint that checks database and Redis connectivity:

```bash
curl -s https://codepilot.your-domain.com/api/health | jq
```

**Response:**

```json
{
  "status": "healthy",
  "version": "20260705",
  "uptime": "24h15m30s",
  "database": "connected",
  "redis": "connected"
}
```

### Docker Health Checks

All services in `docker-compose.yml` include built-in health checks:

| Service | Check Command | Interval | Retries |
|---|---|---|---|
| PostgreSQL | `pg_isready -U codepilot -d codepilot` | 10s | 5 |
| Redis | `redis-cli ping` | 10s | 5 |
| Backend | `wget -qO- http://localhost:8080/api/health` | 30s | 3 |

View health status:

```bash
docker inspect --format='{{json .State.Health}}' codepilot-backend | jq
docker inspect --format='{{json .State.Health}}' codepilot-postgres | jq
```

### External Monitoring

Set up an external uptime monitor (e.g., UptimeRobot, Pingdom, or Datadog) to poll:

```
GET https://codepilot.your-domain.com/api/health
Expected: HTTP 200, body contains "healthy"
Alert if: response time > 5s or status ≠ 200
```

### Log Aggregation

#### Docker JSON Logs

```bash
# Tail all logs
docker compose logs -f --tail=100

# Export to file for analysis
docker compose logs --no-color > /var/log/codepilot/all.log
```

#### Production Log Stack Options

| Stack | Description |
|---|---|
| **ELK** (Elasticsearch + Logstash + Kibana) | Full-featured log pipeline |
| **Loki + Grafana** | Lightweight, Prometheus-like log aggregation |
| **CloudWatch** (AWS) | Native AWS log management |
| **Cloud Logging** (GCP) | Native GCP log management |

The backend uses **zerolog** for structured JSON logging in production mode (`APP_ENVIRONMENT=production`), making it compatible with all major log aggregation tools.

---

## Scaling

### Horizontal Scaling Considerations

#### Backend (Go API)

The Go backend is stateless and can be horizontally scaled:

```yaml
# docker-compose.scale.yml
services:
  backend:
    deploy:
      replicas: 3
```

When running multiple backend replicas, use a load balancer:

```nginx
upstream codepilot_backend {
    least_conn;
    server backend-1:8080;
    server backend-2:8080;
    server backend-3:8080;
}

server {
    location /api/ {
        proxy_pass http://codepilot_backend;
    }
}
```

> [!WARNING]
> The backend spawns GitHub MCP server containers via Docker socket. When scaling horizontally across multiple hosts, ensure each host has Docker installed and the MCP server image pre-pulled.

#### Database Connection Pooling

The backend configures connection pooling via `DatabaseConfig`:

| Parameter | Default | Production Recommendation |
|---|---|---|
| `DB_MAX_OPEN_CONNS` | 25 | `(CPU cores × 2) + spinning_disks` per instance |
| `DB_MAX_IDLE_CONNS` | 5 | 25–50% of max open conns |
| `ConnMaxLifetime` | 30 min | Keep as-is to prevent stale connections |
| `ConnMaxIdleTime` | 5 min | Keep as-is |

For many backend replicas, use **PgBouncer** as a connection pooler in front of PostgreSQL:

```yaml
# Add to docker-compose.yml
pgbouncer:
  image: edoburu/pgbouncer:latest
  environment:
    DATABASE_URL: postgres://codepilot:<password>@postgres:5432/codepilot
    POOL_MODE: transaction
    MAX_CLIENT_CONN: 200
    DEFAULT_POOL_SIZE: 25
  ports:
    - "6432:6432"
  depends_on:
    - postgres
```

Then point `DB_HOST` to `pgbouncer` and `DB_PORT` to `6432`.

#### Redis Scaling

For most deployments, a single Redis instance with AOF persistence is sufficient. For higher availability:

- **Redis Sentinel** — automatic failover with master/replica topology
- **Redis Cluster** — sharded data across multiple nodes (for very high throughput)

#### Frontend Scaling

The frontend is a static React build served by Nginx. It can be served from:

- A CDN (CloudFront, Cloudflare, Vercel)
- Multiple Nginx instances behind a load balancer
- An S3/GCS bucket with CDN origin

---

## Backup Strategy

### PostgreSQL Backups

#### Automated Daily Backups (Cron)

Create `/opt/codepilot-ai/scripts/backup-db.sh`:

```bash
#!/bin/bash
set -euo pipefail

BACKUP_DIR="/opt/codepilot-ai/backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RETENTION_DAYS=30

mkdir -p "$BACKUP_DIR"

# Dump the database
docker compose exec -T postgres pg_dump \
  -U codepilot \
  -d codepilot \
  --format=custom \
  --compress=9 \
  > "${BACKUP_DIR}/codepilot_${TIMESTAMP}.dump"

echo "Backup created: codepilot_${TIMESTAMP}.dump"

# Remove backups older than retention period
find "$BACKUP_DIR" -name "codepilot_*.dump" -mtime +${RETENTION_DAYS} -delete
echo "Cleaned backups older than ${RETENTION_DAYS} days"
```

Add to crontab:

```bash
chmod +x /opt/codepilot-ai/scripts/backup-db.sh

# Run daily at 2:00 AM
crontab -e
0 2 * * * /opt/codepilot-ai/scripts/backup-db.sh >> /var/log/codepilot/backup.log 2>&1
```

#### Manual Backup & Restore

```bash
# Backup
docker compose exec -T postgres pg_dump -U codepilot -d codepilot \
  --format=custom > backup.dump

# Restore (⚠️ destructive — drops existing data)
docker compose exec -T postgres pg_restore -U codepilot -d codepilot \
  --clean --if-exists < backup.dump
```

#### Cloud Backup Options

| Cloud Provider | Service | Strategy |
|---|---|---|
| **AWS** | RDS for PostgreSQL | Automated snapshots + point-in-time recovery |
| **GCP** | Cloud SQL for PostgreSQL | Automated backups + on-demand backups |
| **Azure** | Azure Database for PostgreSQL | Geo-redundant backup storage |

### Redis Backups

Redis is configured with **AOF persistence** (`--appendonly yes`). The data file is stored in the `redis_data` Docker volume.

```bash
# Trigger a manual snapshot
docker compose exec redis redis-cli BGSAVE

# Copy the dump file
docker compose cp redis:/data/dump.rdb ./redis-backup.rdb
```

> [!TIP]
> Redis primarily caches LLM responses and rate-limiting state. Losing Redis data is non-critical — the backend will simply re-query the LLM for cache misses.

---

## Troubleshooting

### Common Issues

#### 1. "failed to ping database" on startup

**Cause:** The backend starts before PostgreSQL is ready.

**Solution:** Docker Compose health checks should prevent this. If it persists:

```bash
# Verify PostgreSQL is healthy
docker compose exec postgres pg_isready -U codepilot -d codepilot

# Check PostgreSQL logs
docker compose logs postgres

# Restart the backend after Postgres is healthy
docker compose restart backend
```

#### 2. Webhook events not received

**Cause:** Payload URL is unreachable from GitHub, or the secret doesn't match.

**Checklist:**
- [ ] Payload URL is publicly accessible (`https://your-domain.com/api/webhook`)
- [ ] Content type is set to `application/json`
- [ ] Webhook secret matches `GITHUB_WEBHOOK_SECRET` in `.env`
- [ ] The "Pull requests" event is selected
- [ ] The webhook is active (enabled)

**Debug:**
```bash
# Check GitHub webhook delivery history:
# Repository → Settings → Webhooks → Recent Deliveries

# Check backend logs for webhook processing
docker compose logs backend | grep -i webhook
```

#### 3. "webhook signature verification failed"

**Cause:** The `GITHUB_WEBHOOK_SECRET` in `.env` doesn't match the secret configured in GitHub.

**Solution:** Ensure both secrets are identical. Re-generate if needed:

```bash
# Generate a new secret
openssl rand -hex 32

# Update both:
# 1. .env → GITHUB_WEBHOOK_SECRET=<new-secret>
# 2. GitHub → Settings → Webhooks → Edit → Secret → <new-secret>

# Restart the backend
docker compose restart backend
```

#### 4. MCP server container fails to start

**Cause:** Docker socket not mounted, or MCP image not pulled.

**Solution:**

```bash
# Verify Docker socket is mounted
docker compose exec backend ls -la /var/run/docker.sock

# Pre-pull the MCP image
docker pull ghcr.io/github/github-mcp-server

# Test MCP server directly
docker run -i --rm \
  -e GITHUB_PERSONAL_ACCESS_TOKEN=ghp_your_token \
  ghcr.io/github/github-mcp-server
```

#### 5. LLM API errors / timeouts

**Cause:** Invalid API key, rate limiting, or network issues.

**Checklist:**
- [ ] `LLM_API_KEY` is valid and has sufficient credits
- [ ] `LLM_PROVIDER` matches the API key type (`openai` or `anthropic`)
- [ ] `LLM_MODEL` is a valid model identifier for the provider

```bash
# Test OpenAI API key
curl https://api.openai.com/v1/models \
  -H "Authorization: Bearer $LLM_API_KEY"
```

#### 6. "connection refused" on port 8080

**Cause:** Backend not running or port conflict.

```bash
# Check if the port is in use
lsof -i :8080

# Check container status
docker compose ps
docker compose logs backend --tail=20
```

#### 7. Database migration failures

**Cause:** Schema conflicts or permissions issues.

```bash
# Check current database state
docker compose exec postgres psql -U codepilot -d codepilot -c "\dt"

# Re-run migrations
make migrate-up

# If a migration is stuck, manually fix and re-apply
docker compose exec postgres psql -U codepilot -d codepilot
```

#### 8. Frontend shows blank page or CORS errors

**Cause:** Backend URL misconfigured, or Nginx not proxying correctly.

```bash
# Check Nginx configuration
sudo nginx -t

# Verify the backend is reachable from the frontend container
docker compose exec frontend wget -qO- http://backend:8080/api/health
```

### Useful Diagnostic Commands

```bash
# Full system status
docker compose ps
docker compose top

# Resource usage per container
docker stats --no-stream

# Disk usage
docker system df

# Network connectivity between containers
docker compose exec backend ping postgres
docker compose exec backend ping redis

# PostgreSQL connection count
docker compose exec postgres psql -U codepilot -d codepilot \
  -c "SELECT count(*) FROM pg_stat_activity;"

# Redis memory usage
docker compose exec redis redis-cli info memory
```

---

## Production Checklist

Before going live, verify every item:

- [ ] `APP_ENVIRONMENT=production` is set
- [ ] Strong, unique `DB_PASSWORD` (≥ 32 characters)
- [ ] Strong, unique `GITHUB_WEBHOOK_SECRET` (≥ 32 hex characters)
- [ ] SSL/TLS configured with valid certificates
- [ ] GitHub webhook points to your public HTTPS URL
- [ ] Database ports **not** exposed externally (remove `ports:` in production compose)
- [ ] Redis port **not** exposed externally
- [ ] Log aggregation configured
- [ ] Database backup cron job running
- [ ] Health check monitoring configured
- [ ] Resource limits set for all containers
- [ ] Non-root user in backend container (already configured in Dockerfile)
- [ ] Docker images rebuilt with `--no-cache` for the latest code
- [ ] DNS records point to your server
- [ ] Firewall allows only ports 80, 443 from the internet
