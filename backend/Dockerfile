FROM golang:1.24-alpine AS builder
WORKDIR /app

ENV GOTOOLCHAIN=auto

# Copy Go workspace files
COPY go.work go.work.sum ./
COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download

# Copy backend source
COPY backend ./backend
RUN cd backend && CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/api ./cmd/api

# Runtime stage
FROM alpine:3.21
WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /app/api ./api

EXPOSE 8080

CMD ["/app/api"]
