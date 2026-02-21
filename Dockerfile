# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev sqlite-dev

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o ./bin/sync-daemon .

# Final stage
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates curl

# Copy binary from builder
COPY --from=builder /app/bin/sync-daemon .

# Create data directory
RUN mkdir -p /app/data && chmod 777 /app/data

# Create non-root user (use UID 1001 in case 1000 is taken)
RUN addgroup -g 1001 daemon && adduser -D -u 1001 -G daemon daemon 2>/dev/null || true

# Set ownership
RUN chown -R daemon:daemon /app/data || true

# Switch to non-root user
USER daemon

# Expose ports
EXPOSE 8080 9090

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Default command
ENTRYPOINT ["/app/sync-daemon"]
CMD ["-config", "/etc/sync/config.yaml", "-db", "/app/data/sync.db"]
