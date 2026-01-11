# Highlights Manager

[![Test](https://github.com/mrlokans/highlights-manager/actions/workflows/test.yml/badge.svg)](https://github.com/mrlokans/highlights-manager/actions/workflows/test.yml)
[![Docker](https://github.com/mrlokans/highlights-manager/actions/workflows/docker.yml/badge.svg)](https://github.com/mrlokans/highlights-manager/actions/workflows/docker.yml)
[![codecov](https://codecov.io/gh/mrlokans/highlights-manager/branch/main/graph/badge.svg)](https://codecov.io/gh/mrlokans/highlights-manager)

A self-hosted service for importing, storing, and exporting book highlights from various sources (Readwise, KOReader, MoonReader) to Obsidian-compatible markdown files.

# DISCLAIMER

The project is in a very early stage. Should not be expected to work properly :) .

## Quick Start

```bash
# 1. Clone and enter directory
git clone <repo-url> && cd book-highlights-manager

# 2. Create a directory for your Obsidian vault (or use existing)
mkdir -p ./vault

# 3. Start the service
docker compose up -d

# 4. Open http://localhost:8080 to view your highlights
```

## Features

- Import highlights from Readwise, KOReader, and MoonReader
- Export to Obsidian-compatible markdown with YAML frontmatter
- Web UI for browsing and downloading highlights
- SQLite database for persistent storage
- Docker-ready with health checks
- REST API for automation

## Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `OBSIDIAN_VAULT_DIR` | **Required.** Path to Obsidian vault directory | - |
| `OBSIDIAN_EXPORT_PATH` | Subfolder within vault for highlights | `BookHighlights` |
| `DATABASE_PATH` | Path to SQLite database file | `./highlights-manager.db` |
| `AUDIT_DIR` | Directory for audit logs | `./audit` |
| `HOST` | Server bind address | `0.0.0.0` |
| `PORT` | Server port | `8080` |
| `READWISE_TOKEN` | Readwise API token (optional) | - |
| `DROPBOX_APP_KEY` | Dropbox app key for MoonReader sync (optional) | - |
| `TOKEN_ENCRYPTION_KEY` | **Required for OAuth features.** Base64-encoded 32-byte key for encrypting OAuth tokens. Generate with: `openssl rand -base64 32` | - |

## Deployment

### Docker Compose (Recommended)

Create a `.env` file:
```bash
OBSIDIAN_VAULT_DIR=/path/to/your/obsidian/vault
READWISE_TOKEN=your_token_here  # optional
```

Then run:
```bash
docker compose up -d
```

Data is persisted in `./data` directory (database + audit logs).

### Docker CLI

```bash
docker run -d \
  --name highlights-manager \
  -p 8080:8080 \
  -v ./data:/data \
  -v /path/to/vault:/vault \
  -e OBSIDIAN_VAULT_DIR=/vault \
  ghcr.io/mrlokans/highlights-manager
```

### Binary

Download from releases or build from source:
```bash
make build
./build/highlights-manager-darwin
```

## Volume Mapping

| Container Path | Purpose | Required |
|----------------|---------|----------|
| `/data` | Database and audit logs | Yes |
| `/vault` | Obsidian vault for markdown export | Yes |

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check with DB status |
| `/api/books` | GET | List all books |
| `/api/books/search` | GET | Search by title/author |
| `/api/books/stats` | GET | Database statistics |
| `/import/koreader` | POST | Import KOReader JSON |
| `/api/v2/highlights` | POST | Import Readwise highlights |
| `/import/moonreader` | POST | Import MoonReader backup |

## Backup

SQLite database can be backed up while running:
```bash
sqlite3 ./data/highlights-manager.db ".backup backup.db"
```

Or with docker:
```bash
docker exec highlights-manager sqlite3 /data/highlights-manager.db ".backup /data/backup.db"
```

## Health Check

The `/health` endpoint returns:
```json
{
  "status": "healthy",
  "time": "2024-01-11T10:30:00Z",
  "version": "v1.0.0",
  "checks": {
    "database": "ok"
  }
}
```

## Development

```bash
# Run locally
make local

# Run tests
make test

# Build with version info
make build
```

## License

MIT
