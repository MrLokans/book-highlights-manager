# Development

## Prerequisites

- [Go](https://go.dev/dl/) 1.27+ (`go version`)
- [golangci-lint](https://golangci-lint.run/welcome/install/) and `goimports` (`go install golang.org/x/tools/cmd/goimports@latest`) for lint and format
- Docker for image builds
- [prek](https://github.com/j178/prek) for pre-commit hooks

All commands run from the repository root.

## Commands

```bash
# Run locally on http://localhost:8188, database at ./highlights-manager.db
make local

# Run with authentication
make run-auth

# Run tests (results go to test-output.json; the terminal shows only coverage)
make test

# Lint
make lint

# Format (go fmt + goimports + golangci-lint --fix)
make fmt

# Build binary
make build

# Build a local Docker image
docker build -t highlights-manager .
```

`make build-image` is for maintainers only: it builds both platforms and **pushes** to `ghcr.io/mrlokans/highlights-manager:latest`.

## Pre-commit hooks

After installing prek itself, enable the hooks with:

```bash
prek install
```

Hooks run on every commit:
- **Whitespace & file fixes** — trailing whitespace, end-of-file fixer, large file check, merge conflict check
- **`make fmt`** — go fmt, goimports, golangci-lint --fix
- **`make lint`** — golangci-lint
- **`go test`** — unit tests (`-short` mode)

End-to-end tests are described in [e2e/README.md](e2e/README.md).
