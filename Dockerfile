# syntax=docker/dockerfile:1

# Use the official Golang image as the builder
FROM --platform=$BUILDPLATFORM golang:1.23 AS builder

ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if unchanged
RUN go mod download

# Copy the source code into the container
COPY . .

# Build with version information embedded
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags="-w -s -X main.Version=${VERSION} -X main.Commit=${COMMIT}" \
    -o /highlights-manager

# Start from debian slim for better tooling support (wget for healthchecks)
FROM --platform=$TARGETPLATFORM debian:bookworm-slim AS release

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
ENV DATABASE_PATH=/data/highlights-manager.db
ENV AUDIT_DIR=/data/audit
ENV TEMPLATES_PATH=/app/templates
ENV STATIC_PATH=/app/static

EXPOSE 8080

# Healthcheck using wget
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD wget -q --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/highlights-manager"]