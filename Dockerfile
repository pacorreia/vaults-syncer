# Build stage
FROM golang:1.26.3-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev sqlite-dev

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build arguments for version
ARG VERSION=dev
ARG BUILD_DATE
ARG GIT_COMMIT

# Build target arguments provided by buildx
ARG TARGETOS
ARG TARGETARCH

# Build the application with version information
# CGO_LDFLAGS=-static produces a fully static binary so the Alpine final
# stage doesn't need libgcc_s or sqlite-libs at runtime.
RUN CGO_ENABLED=1 CGO_LDFLAGS="-static" GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build \
    -a -installsuffix cgo \
    -ldflags "-extldflags '-static' -X main.Version=${VERSION} -X main.BuildDate=${BUILD_DATE} -X main.GitCommit=${GIT_COMMIT}" \
    -o ./bin/sync-daemon .

# Final stage — no apk installs needed:
#  • binary is statically linked (no libgcc_s / sqlite-libs required)
#  • CA certs copied from builder (no network call)
#  • healthcheck uses busybox wget (built into Alpine)
FROM alpine:3.23

WORKDIR /app

# Copy CA certificates from builder so HTTPS vault calls work
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

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

# Health check — wget is part of busybox, no extra package needed
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

# All configuration is provided via environment variables (DB_TYPE, DB_PATH, MASTER_ENCRYPTION_KEY, etc.)
ENTRYPOINT ["/app/sync-daemon"]
