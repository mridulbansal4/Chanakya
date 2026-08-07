# ==========================================
# 1. Build Go Backend
# ==========================================
FROM golang:1.24-alpine AS backend-builder
WORKDIR /app

# Enable Go toolchain auto-download for Go >= 1.25 requirements
ENV GOTOOLCHAIN=auto

# Copy Go workspace files
COPY go.work go.work.sum ./
COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download

# Copy backend source code
COPY backend ./backend
RUN cd backend && CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/api ./cmd/api

# ==========================================
# 2. Build Next.js Frontend
# ==========================================
FROM node:22-alpine AS frontend-builder
WORKDIR /app

# Copy root & frontend package files
COPY package.json package-lock.json ./
COPY frontend/package.json frontend/package-lock.json frontend/turbo.json ./frontend/
COPY frontend/apps/web/package.json ./frontend/apps/web/
COPY frontend/packages/ui/package.json ./frontend/packages/ui/

# Install root & frontend workspace dependencies
RUN npm ci && cd frontend && npm ci

# Copy full source
COPY . .

# Build Next.js in standalone mode
ENV NEXT_TELEMETRY_DISABLED=1
ENV NODE_ENV=production
RUN cd frontend && npm run build

# ==========================================
# 3. Final Production Container (Cloud Run / Offline)
# ==========================================
FROM node:22-alpine AS runner
WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=3000
ENV HOSTNAME="0.0.0.0"

# Copy Go binary from backend-builder
COPY --from=backend-builder /app/api ./api

# Copy standalone Next.js server & static assets
COPY --from=frontend-builder /app/frontend/apps/web/.next/standalone ./
COPY --from=frontend-builder /app/frontend/apps/web/.next/static ./frontend/apps/web/.next/static
COPY --from=frontend-builder /app/frontend/apps/web/public ./frontend/apps/web/public

# Copy entrypoint script
COPY docker-entrypoint.sh ./
RUN chmod +x ./docker-entrypoint.sh

EXPOSE 3000

ENTRYPOINT ["/app/docker-entrypoint.sh"]
