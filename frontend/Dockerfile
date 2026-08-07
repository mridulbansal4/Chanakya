FROM node:22-alpine AS builder
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

# Runtime stage
FROM node:22-alpine AS runner
WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=3000
ENV HOSTNAME="0.0.0.0"

# Copy standalone Next.js server & static assets
COPY --from=builder /app/frontend/apps/web/.next/standalone ./
COPY --from=builder /app/frontend/apps/web/.next/static ./frontend/apps/web/.next/static
COPY --from=builder /app/frontend/apps/web/public ./frontend/apps/web/public

EXPOSE 3000

CMD ["node", "frontend/apps/web/server.js"]
