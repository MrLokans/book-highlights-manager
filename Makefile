.PHONY: build-image build build-local run local clean test test_coverage test-auth dep lint check run-auth demo generate-demo

BUILDER_NAME := exporter-container

REGISTRY_URL := ghcr.io
IMAGE_NAME := highlights-manager
GITHUB_USERNAME := mrlokans
REMOTE_IMAGE_IDENTIFIER := $(REGISTRY_URL)/$(GITHUB_USERNAME)/$(IMAGE_NAME)

LOCAL_BUILD_DIR := build
BINARY_NAME := highlights-manager
BUILD_PATH := $(LOCAL_BUILD_DIR)/$(BINARY_NAME)

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_FLAGS := -ldflags="-w -s -X main.Version=$(VERSION) -X main.Commit=$(COMMIT)"

# Include env file if it exists
-include .env-local
export $(shell [ -f .env-local ] && sed 's/=.*//' .env-local)

build-image:
	docker buildx create --name $(BUILDER_NAME) --driver=docker-container || true
	docker buildx build -f Dockerfile -t $(REMOTE_IMAGE_IDENTIFIER):latest \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--platform linux/amd64,linux/arm64/v8 \
		--builder $(BUILDER_NAME) --push .

build:
	GOARCH=amd64 GOOS=darwin go build $(BUILD_FLAGS) -o ${BUILD_PATH}-darwin main.go

build-local:
	CGO_ENABLED=1 GOARCH=amd64 GOOS=darwin go build $(BUILD_FLAGS) -o ${BUILD_PATH}-darwin main.go


run: build-local
	./${BUILD_PATH}-darwin


local:
	go run main.go


clean:
	go clean
	rm ${BUILD_PATH}-darwin || true
	rm ${BUILD_PATH}-linux || true
	rm ${BUILD_PATH}-windows || true

test:
	go test ./... -cover -coverprofile=coverage.out -covermode=atomic -json > test-output.json
	@echo ""
	@echo "Coverage summary:"
	@go tool cover -func=coverage.out | tail -1

test_coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

dep:
	go mod download

lint:
	go build ./...
	golangci-lint run --timeout=5m

# Pre-commit check: runs lint and tests
check: lint test
	@echo "All checks passed!"

# Auth-specific targets
test-auth:
	go test ./internal/auth/... -v -cover

# Run with authentication enabled (local mode)
run-auth: build-local
	AUTH_MODE=local ./${BUILD_PATH}-darwin

# Demo mode targets
generate-demo:
	go run cmd/generate_demo/main.go

# Run in demo mode (regenerates demo database first, ignores .env-local settings)
# Since writes are blocked, we use demo.db directly (no separate live.db needed)
demo: generate-demo
	env -i PATH="$$PATH" HOME="$$HOME" \
	DEMO_MODE=true \
	DEMO_DB_PATH=./demo/demo.db \
	DATABASE_PATH=./demo/demo.db \
	AUTH_MODE=none \
	OBSIDIAN_VAULT_DIR=./demo/vault \
	TEMPLATES_PATH=./templates \
	STATIC_PATH=./static \
	go run main.go