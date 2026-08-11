# Studio UI build stage — produces web/dist, embedded into the server binary.
FROM node:22-alpine AS studio
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build && touch dist/.gitkeep

# Guides docs site build stage — produces website/dist (the Starlight site served
# at /guides), embedded into the server binary. The build syncs from ../knowledge,
# so the knowledge/ bundle is copied in before building.
FROM node:22-alpine AS guides
WORKDIR /app/website
COPY website/package.json website/package-lock.json ./
RUN npm ci
COPY website/ ./
COPY knowledge/ /app/knowledge/
RUN npm run build:embed && touch dist/.gitkeep

# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Overlay the built Studio UI and docs site so //go:embed picks up the real
# assets (not the checked-in .gitkeep placeholders).
COPY --from=studio /app/web/dist ./web/dist
COPY --from=guides /app/website/dist ./website/dist

# Generate API code and build all binaries
RUN go generate ./api && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/gexbot-server ./cmd/server && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/gexbot-daemon ./cmd/daemon && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/gexbot-downloader ./cmd/downloader

# Runtime stage - Server
FROM alpine:3.21 AS server

WORKDIR /app

# Create non-root user
RUN addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -D appuser

# Copy binary from builder
COPY --from=builder /app/bin/gexbot-server /app/gexbot-server

# Create data directory
RUN mkdir -p /app/data && chown -R appuser:appgroup /app

EXPOSE 8080

# Default to non-root user (can be overridden by docker-compose user directive)
USER appuser

ENTRYPOINT ["/app/gexbot-server"]

# Runtime stage - Daemon
FROM alpine:3.21 AS daemon

WORKDIR /app

# Install tzdata for timezone support (critical for scheduling)
RUN apk add --no-cache tzdata && \
    addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -D appuser

# Copy binary and default config from builder
COPY --from=builder /app/bin/gexbot-daemon /app/gexbot-daemon
COPY --from=builder /app/configs/default.yaml /app/configs/default.yaml

# Create directories
RUN mkdir -p /app/data /app/logs && chown -R appuser:appgroup /app

# Default to non-root user (can be overridden by docker-compose user directive)
USER appuser

ENTRYPOINT ["/app/gexbot-daemon"]

# Runtime stage - Tools (one-off downloader CLI: manual backfills + eod pack/verify/materialize migration)
FROM alpine:3.21 AS tools

WORKDIR /app

RUN apk add --no-cache tzdata && \
    addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -D appuser

COPY --from=builder /app/bin/gexbot-downloader /app/gexbot-downloader
COPY --from=builder /app/configs/default.yaml /app/configs/default.yaml

RUN mkdir -p /app/data /app/logs && chown -R appuser:appgroup /app

USER appuser

ENTRYPOINT ["/app/gexbot-downloader"]
