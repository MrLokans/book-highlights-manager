# Self-hosting

This picks up after the [Quick start](../README.md#install): how to secure, back up and troubleshoot a running instance.

## Authentication

Enable authentication for multi-user deployments or external access by adding these to the `environment:` list in your `docker-compose.yml`, then running `docker compose up -d` again:

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

On first run with `AUTH_MODE=local`, open `http://localhost:8080/setup` in a browser (the form is CSRF-protected, so curl won't work) and choose a username and password for the administrator account (the home page redirects to `/login`, which won't work until an account exists).

## OAuth token encryption

If using Dropbox sync for Moon+ Reader, encrypt stored tokens:
```yaml
environment:
  - TOKEN_ENCRYPTION_KEY=<generate-with-openssl-rand-base64-32>
```

Dropbox app setup is described in [oauth2-setup.md](oauth2-setup.md).

## Production compose

Compared to the quick start, this adds authentication, optional integrations, a real vault path and secrets from `.env`:

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
      - OBSIDIAN_VAULT_DIR=/vault

      # Authentication (recommended for external access)
      - AUTH_MODE=local
      - AUTH_SESSION_SECRET=${AUTH_SESSION_SECRET}
      - AUTH_SECURE_COOKIES=true

      # Optional integrations
      - READWISE_TOKEN=${READWISE_TOKEN:-}
      - DROPBOX_APP_KEY=${DROPBOX_APP_KEY:-}
      - TOKEN_ENCRYPTION_KEY=${TOKEN_ENCRYPTION_KEY:-}
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s
```

Create a `.env` file for secrets next to `docker-compose.yml` (never commit this). Compose does **not** run shell commands inside `.env`, so generate the values with your shell:
```bash
cat > .env <<EOF
AUTH_SESSION_SECRET=$(openssl rand -base64 32)
TOKEN_ENCRYPTION_KEY=$(openssl rand -base64 32)
READWISE_TOKEN=your_token_here
EOF
```
Check that `.env` contains real random strings, not `$(openssl ...)`.

All other settings are listed in [configuration.md](configuration.md).

## Volumes

| Container Path | Purpose | Required |
|----------------|---------|----------|
| `/data` | Database, audit logs | Yes |
| `/vault` | Obsidian vault for exports | No |

## Backup & restore

**Backup the database:**
```bash
# From the host (needs the sqlite3 CLI; the container image does not include it)
sqlite3 ./data/highlights-manager.db ".backup backup.db"
```

**Restore:**
```bash
docker compose down
cp backup.db ./data/highlights-manager.db
docker compose up -d
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
- Use `wget -qO-` as in the examples above; `wget --spider` sends HEAD, which `/health` doesn't answer
- Wait for `start_period` (10s) after container start
- Check logs: `docker compose logs highlights-manager`
