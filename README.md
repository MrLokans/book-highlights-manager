# Highlights Manager

[![Test](https://github.com/MrLokans/book-highlights-manager/actions/workflows/test.yml/badge.svg)](https://github.com/MrLokans/book-highlights-manager/actions/workflows/test.yml)
[![Docker](https://github.com/MrLokans/book-highlights-manager/actions/workflows/docker.yml/badge.svg)](https://github.com/MrLokans/book-highlights-manager/actions/workflows/docker.yml)
[![codecov](https://codecov.io/gh/MrLokans/book-highlights-manager/branch/main/graph/badge.svg)](https://codecov.io/gh/MrLokans/book-highlights-manager)

A self-hosted service for importing, managing, and exporting book highlights from Kindle, Apple Books, Moon+ Reader, and Readwise to Obsidian-compatible markdown.

## Demo

To spare the details on descriptions and screenshots - here's the live demo, where some public domain data is uploaded and RO-only access is provided - [link](https://highlights-demo.mrlokans.work/).

## Self-Hosting Guide

### Quick Start with Docker (Recommended)

**1. Create a directory structure:**
```bash
mkdir -p highlights-manager/{data,vault}
cd highlights-manager
```

**2. Create a `docker-compose.yml`:**
```yaml
services:
  highlights-manager:
    image: ghcr.io/mrlokans/highlights-manager:latest
    container_name: highlights-manager
    restart: unless-stopped
    ports:
      - "127.0.0.1:8080:8080"  # Bind to localhost only
    volumes:
      - ./data:/data
      - ./vault:/vault
    environment:
      - OBSIDIAN_VAULT_DIR=/vault
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

**3. Start the service:**
```bash
docker compose up -d
```

**4. Open http://localhost:8080**

### Security Considerations

#### Authentication

Enable authentication for multi-user deployments or external access:

```yaml
environment:
  - AUTH_MODE=local
  - AUTH_SESSION_SECRET=<generate-with-openssl-rand-base64-32>
  - AUTH_SECURE_COOKIES=true  # Requires HTTPS
```

Generate a secure session secret:
```bash
openssl rand -base64 32
```

On first run with `AUTH_MODE=local`, visit `/setup` to create the administrator account.

#### OAuth Token Encryption

If using Dropbox sync for Moon+ Reader, encrypt stored tokens:
```yaml
environment:
  - TOKEN_ENCRYPTION_KEY=<generate-with-openssl-rand-base64-32>
```

### Production Docker Compose

Complete example for production deployment:

```yaml
services:
  highlights-manager:
    image: ghcr.io/mrlokans/highlights-manager:latest
    container_name: highlights-manager
    restart: unless-stopped
    ports:
      - "127.0.0.1:8080:8080"
    volumes:
      - ./data:/data
      # Optional for automatic sync to work
      - /path/to/your/obsidian/vault:/vault
    environment:

      # Authentication (recommended for external access)
      - AUTH_MODE=local
      - AUTH_SESSION_SECRET=${AUTH_SESSION_SECRET}
      - AUTH_SECURE_COOKIES=true

      # Optional integrations
      - READWISE_TOKEN=${READWISE_TOKEN:-}
      - DROPBOX_APP_KEY=${DROPBOX_APP_KEY:-}
      - TOKEN_ENCRYPTION_KEY=${TOKEN_ENCRYPTION_KEY:-}
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s
```

Create a `.env` file for secrets (never commit this):
```bash
AUTH_SESSION_SECRET=$(openssl rand -base64 32)
TOKEN_ENCRYPTION_KEY=$(openssl rand -base64 32)
READWISE_TOKEN=your_token_here
```

## Features

### Import Sources

| Source | Method | Notes |
|--------|--------|-------|
| **Kindle** | Upload `My Clippings.txt` | Via web UI or API |
| **Apple Books** | CLI command | macOS only, reads local databases |
| **Moon+ Reader** | Dropbox sync or file upload | Supports highlight colors/styles |
| **Readwise** | API webhook or CSV import | Requires API token |

### Export

- **Obsidian markdown** with YAML frontmatter (title, author, tags, highlights count)
- **Download individual books** or **bulk ZIP export** via web UI
- Configurable export directory via `OBSIDIAN_EXPORT_DIR`

### Web UI

- Browse and search books and highlights
- Tag management with autocomplete
- Book cover display (fetched from OpenLibrary)
- Mark favorite highlights
- Download highlights as markdown

### Metadata Enrichment

- Automatic book metadata lookup via OpenLibrary
- ISBN, publisher, publication year, cover images
- Bulk enrichment for existing library

### Other Features

- **Vocabulary tracking**: Extract and look up word definitions from you highlights

## Configuration Reference

### Core Settings

| Variable | Description | Default |
|----------|-------------|---------|
| `OBSIDIAN_EXPORT_DIR` | Directory for markdown exports | - |
| `DATABASE_PATH` | SQLite database location | `/data/highlights-manager.db` (Docker) |
| `HOST` | Bind address | `0.0.0.0` |
| `PORT` | Server port | `8080` (Docker), `8188` (local) |
| `AUDIT_RETENTION_DAYS` | Days to keep audit events in database | `30` |

### Obsidian Sync

Automatically export highlights to your Obsidian vault on a schedule.

| Variable | Description | Default |
|----------|-------------|---------|
| `OBSIDIAN_SYNC_ENABLED` | Enable automatic sync | `false` |
| `OBSIDIAN_SYNC_SCHEDULE` | Cron schedule for sync | `0 * * * *` (hourly) |

### Authentication

| Variable | Description | Default |
|----------|-------------|---------|
| `AUTH_MODE` | `none` or `local` | `none` |
| `AUTH_SESSION_SECRET` | 32-byte base64 key | Auto-generated |
| `AUTH_SESSION_LIFETIME` | Session duration | `24h` |
| `AUTH_TOKEN_EXPIRY` | API token lifetime | `720h` |
| `AUTH_SECURE_COOKIES` | HTTPS-only cookies | `true` |
| `AUTH_BCRYPT_COST` | Password hash cost | `12` |
| `AUTH_MAX_LOGIN_ATTEMPTS` | Lockout threshold | `5` |
| `AUTH_RATE_LIMIT_WINDOW` | Window for counting failed attempts | `15m` |
| `AUTH_LOCKOUT_DURATION` | Lockout duration | `30m` |

### Integrations

| Variable | Description | Default |
|----------|-------------|---------|
| `READWISE_TOKEN` | Readwise API token | - |
| `DROPBOX_APP_KEY` | Dropbox app key for Moon+ Reader | - |
| `TOKEN_ENCRYPTION_KEY` | AES-256 key for OAuth tokens | Auto-generated |

### Background Tasks

| Variable | Description | Default |
|----------|-------------|---------|
| `TASKS_ENABLED` | Enable task queue | `true` |
| `TASK_WORKERS` | Concurrent workers | `2` |
| `TASK_TIMEOUT` | Task timeout | `5m` |
| `TASK_MAX_RETRIES` | Max retry attempts | `3` |
| `TASK_RETRY_DELAY` | Delay between retries | `1m` |

### Analytics (Optional)

| Variable | Description | Default |
|----------|-------------|---------|
| `PLAUSIBLE_DOMAIN` | Domain registered in Plausible | - |
| `PLAUSIBLE_SCRIPT_URL` | Plausible script URL | `https://plausible.io/js/script.js` |
| `PLAUSIBLE_EXTENSIONS` | Comma-separated extensions | - |

## API Reference

### Health & Status

```bash
# Health check (useful for monitoring)
curl http://localhost:8080/health
```

### Books

```bash
# List all books
curl http://localhost:8080/api/books

# Search books
curl "http://localhost:8080/api/books/search?title=sapiens&author=harari"

# Get statistics
curl http://localhost:8080/api/books/stats

# Enrich book metadata
curl -X POST http://localhost:8080/api/books/123/enrich
```

### Imports

```bash
# Import Kindle clippings
curl -X POST http://localhost:8080/import/kindle \
  -F "file=@My Clippings.txt"

```

### Tags

```bash
# List tags
curl http://localhost:8080/api/tags

# Add tag to book
curl -X POST http://localhost:8080/api/books/123/tags \
  -H "Content-Type: application/json" \
  -d '{"tag_id": 456}'
```

## Volume Mapping

| Container Path | Purpose | Required |
|----------------|---------|----------|
| `/data` | Database, audit logs | Yes |
| `/vault` | Obsidian vault for exports | No |

## Backup & Restore

**Backup the database:**
```bash
# From host (if data directory is mounted)
sqlite3 ./data/highlights-manager.db ".backup backup.db"

# From container
docker exec highlights-manager sqlite3 /data/highlights-manager.db ".backup /data/backup.db"
```

**Restore:**
```bash
docker compose down
cp backup.db ./data/highlights-manager.db
docker compose up -d
```

## CLI Commands

The binary supports additional CLI commands for local imports:

```bash
# Apple Books import (macOS only)
./highlights-manager applebooks-import

# Kindle import from file
./highlights-manager kindle-import -file "/path/to/My Clippings.txt"

# Moon+ Reader from local filesystem
./highlights-manager moonreader-sync

# Moon+ Reader from Dropbox
./highlights-manager moonreader-dropbox
```

## Demo Mode

Try the service with sample data:

```bash
docker run -p 8080:8080 \
  -e DEMO_MODE=true \
  -e DEMO_USE_EMBEDDED=true \
  ghcr.io/mrlokans/highlights-manager:latest
```

Demo mode uses embedded sample data and blocks write operations.

## Development

```bash
# Run locally
make local

# Run with authentication
make run-auth

# Run tests
make test

# Build binary
make build

# Build Docker image
make build-image
```

## Troubleshooting

**Container won't start:**
- Check volume permissions: directories should be writable by UID 1000

**Can't access from browser:**
- If using `127.0.0.1:8080`, access from the host machine only
- Check if port 8080 is already in use

**Authentication issues:**
- Ensure `AUTH_SESSION_SECRET` is the same across restarts
- Set `AUTH_SECURE_COOKIES=false` if not using HTTPS

**Health check failing:**
- Wait for `start_period` (10s) after container start
- Check logs: `docker compose logs highlights-manager`

## License

MIT
