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

# Generate API code and build both binaries
RUN go generate ./api && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/gexbot-server ./cmd/server && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/gexbot-daemon ./cmd/daemon

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
