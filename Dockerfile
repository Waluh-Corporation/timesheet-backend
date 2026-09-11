# ==============================================================================
# Stage 1: Build binary statically
# ==============================================================================
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Download dependencies with cache optimization
COPY go.mod go.sum ./
RUN go mod download

# Copy application source code
COPY . .

# Build static Go binary with stripped debug symbols and trimmed paths
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -trimpath \
    -o /app/main .

# ==============================================================================
# Stage 2: Minimal & Secure Production Runtime
# ==============================================================================
FROM alpine:3.21

# OCI Image Annotations
LABEL org.opencontainers.image.source="https://github.com/Waluh-Corporation/timesheet-backend"

# ca-certificates for outbound HTTPS/SMTP TLS connections
# tzdata for Asia/Jakarta timezone support in cron scheduler
# wget is built-in to Busybox for container HEALTHCHECK
RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -g 10001 -S appgroup && \
    adduser -u 10001 -S appuser -G appgroup

WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/main /app/main

# Copy required runtime templates and Swagger documentation
COPY --from=builder /app/templates /app/templates
COPY --from=builder /app/docs /app/docs

# Set ownership to unprivileged user
RUN chown -R appuser:appgroup /app

# Run as non-root user
USER appuser:appgroup

# Service port
EXPOSE 8080

# Environment defaults
ENV PORT=8080 \
    GIN_MODE=release \
    TZ=Asia/Jakarta

# Container healthcheck
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/v1/setup/status || exit 1

ENTRYPOINT ["/app/main"]
