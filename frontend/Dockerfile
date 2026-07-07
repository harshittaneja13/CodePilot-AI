# ============================================================================
# CodePilot AI — Frontend Dockerfile
# Multi-stage build: Node for building, Nginx for serving
# ============================================================================

# ---------------------------------------------------------------------------
# Stage 1: Build the React (Vite) application
# ---------------------------------------------------------------------------
FROM node:20-alpine AS builder

WORKDIR /app

# Copy package files first for optimal layer caching.
# npm ci is preferred over npm install for reproducible builds.
COPY package.json package-lock.json* yarn.lock* pnpm-lock.yaml* ./

# Install dependencies — use npm ci for deterministic installs.
# Fall back to npm install if no lock file exists.
RUN if [ -f package-lock.json ]; then npm ci --frozen-lockfile; \
    elif [ -f yarn.lock ]; then yarn install --frozen-lockfile; \
    elif [ -f pnpm-lock.yaml ]; then corepack enable && pnpm install --frozen-lockfile; \
    else npm install; fi

# Copy the rest of the frontend source code.
COPY . .

# Build the production bundle.
# Vite outputs to dist/ by default.
RUN npm run build

# ---------------------------------------------------------------------------
# Stage 2: Serve with Nginx
# nginx:alpine is ~25MB and battle-tested for static file serving.
# ---------------------------------------------------------------------------
FROM nginx:alpine

# Labels for container metadata.
LABEL maintainer="CodePilot AI Team"
LABEL org.opencontainers.image.title="codepilot-frontend"
LABEL org.opencontainers.image.description="CodePilot AI — React Dashboard"

# Remove the default Nginx configuration and static files.
RUN rm -rf /etc/nginx/conf.d/default.conf /usr/share/nginx/html/*

# Copy our custom Nginx configuration.
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Copy the built static assets from the builder stage.
COPY --from=builder /app/dist /usr/share/nginx/html

# Expose port 80 for HTTP traffic.
EXPOSE 80

# Nginx runs in the foreground (required for Docker).
CMD ["nginx", "-g", "daemon off;"]
