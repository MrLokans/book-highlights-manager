# Configuration

All settings are environment variables. In Docker, set them under `environment:` in `docker-compose.yml`; locally, export them before `make local`. Some settings can also be changed in the web UI under **Settings**.

## Core Settings

| Variable | Description | Default |
|----------|-------------|---------|
| `OBSIDIAN_EXPORT_DIR` | Directory for markdown exports (`OBSIDIAN_VAULT_DIR` is accepted as an alias) | - |
| `DATABASE_PATH` | SQLite database location | `/data/highlights-manager.db` (Docker), `./highlights-manager.db` (local) |
| `HOST` | Bind address | `0.0.0.0` |
| `PORT` | Server port | `8080` (Docker), `8188` (local) |
| `AUDIT_RETENTION_DAYS` | Days to keep audit events in database | `30` |

## Obsidian Sync

Automatically export highlights to your Obsidian vault on a schedule.

| Variable | Description | Default |
|----------|-------------|---------|
| `OBSIDIAN_SYNC_ENABLED` | Enable automatic sync | `false` |
| `OBSIDIAN_SYNC_SCHEDULE` | Cron schedule for sync | `0 * * * *` (hourly) |

## Authentication

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

## Integrations

| Variable | Description | Default |
|----------|-------------|---------|
| `READWISE_TOKEN` | Readwise API token | - |
| `DROPBOX_APP_KEY` | Dropbox app key for Moon+ Reader | - |
| `TOKEN_ENCRYPTION_KEY` | AES-256 key for OAuth tokens | Auto-generated |

## Background Tasks

| Variable | Description | Default |
|----------|-------------|---------|
| `TASKS_ENABLED` | Enable task queue | `true` |
| `TASK_WORKERS` | Concurrent workers | `2` |
| `TASK_TIMEOUT` | Task timeout | `5m` |
| `TASK_MAX_RETRIES` | Max retry attempts | `3` |
| `TASK_RETRY_DELAY` | Delay between retries | `1m` |

## Analytics (Optional)

| Variable | Description | Default |
|----------|-------------|---------|
| `PLAUSIBLE_DOMAIN` | Domain registered in Plausible | - |
| `PLAUSIBLE_SCRIPT_URL` | Plausible script URL | `https://plausible.io/js/script.js` |
| `PLAUSIBLE_EXTENSIONS` | Comma-separated extensions | - |
