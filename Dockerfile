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

# Generate API code and build
RUN go generate ./api && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/gexbot-server ./cmd/server

# Runtime stage
FROM alpine:3.21

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
