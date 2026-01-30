# syntax=docker/dockerfile:1

# Use the official Golang image as the builder
FROM --platform=$BUILDPLATFORM golang:1.23 AS builder

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown

# Install cross-compilation toolchains for CGO
# Only install what's needed based on build platform
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc-aarch64-linux-gnu \
    gcc-x86-64-linux-gnu \
    libc6-dev-arm64-cross \
    libc6-dev-amd64-cross \
    && rm -rf /var/lib/apt/lists/*

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if unchanged
RUN go mod download

# Copy the source code into the container
# Demo assets in internal/demo/assets/ are embedded via go:embed
COPY . .

# Build with CGO enabled for sqlite3 support
# Set the appropriate C compiler based on target architecture
RUN set -ex; \
    if [ "${TARGETARCH}" = "amd64" ]; then \
        CC=x86_64-linux-gnu-gcc; \
    else \
        CC=aarch64-linux-gnu-gcc; \
    fi; \
    CGO_ENABLED=1 GOOS=${TARGETOS} GOARCH=${TARGETARCH} CC=${CC} \
    go build \
    -ldflags="-w -s -X main.Version=${VERSION} -X main.Commit=${COMMIT}" \
    -o /highlights-manager

# Start from debian slim for better tooling support (wget for healthchecks)
FROM debian:bookworm-slim AS release

# Install wget for healthchecks and ca-certificates for HTTPS
RUN apt-get update && apt-get install -y --no-install-recommends \
    wget \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user for security
RUN useradd -r -u 1000 -s /sbin/nologin highlights-manager

# Create data directories
RUN mkdir -p /data/audit && chown -R highlights-manager:highlights-manager /data

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /highlights-manager /app/highlights-manager

# Copy static assets
COPY --from=builder /app/templates /app/templates
COPY --from=builder /app/static /app/static

# Set ownership
RUN chown -R highlights-manager:highlights-manager /app

# Switch to non-root user
USER highlights-manager

# Environment defaults for containerized deployment
ENV HOST=0.0.0.0
ENV PORT=8080
ENV GIN_MODE=release
ENV DATABASE_PATH=/data/highlights-manager.db
ENV AUDIT_DIR=/data/audit
ENV TEMPLATES_PATH=/app/templates
ENV STATIC_PATH=/app/static

# Demo mode environment variables (disabled by default)
# Set DEMO_MODE=true and DEMO_USE_EMBEDDED=true to run in demo mode with embedded assets
ENV DEMO_MODE=false
ENV DEMO_USE_EMBEDDED=false

EXPOSE 8080

# Healthcheck using wget
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD wget -q --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/highlights-manager"]