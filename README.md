# Highlights Manager

[![Test](https://github.com/MrLokans/book-highlights-manager/actions/workflows/test.yml/badge.svg)](https://github.com/MrLokans/book-highlights-manager/actions/workflows/test.yml)
[![Docker](https://github.com/MrLokans/book-highlights-manager/actions/workflows/docker.yml/badge.svg)](https://github.com/MrLokans/book-highlights-manager/actions/workflows/docker.yml)
[![codecov](https://codecov.io/gh/MrLokans/book-highlights-manager/branch/main/graph/badge.svg)](https://codecov.io/gh/MrLokans/book-highlights-manager)

Your book highlights are scattered across reading apps. Highlights Manager collects them in one self-hosted library and exports them to your Obsidian vault as Markdown, one note per book.

- **Imports from:** Kindle (`My Clippings.txt`), Apple Books, Moon+ Reader, Readwise
- **In the library:** browse and search highlights, tag books, mark favourites, fill in covers and metadata from OpenLibrary, build a vocabulary list from words you highlighted
- **Exports to:** Obsidian-ready Markdown with YAML frontmatter, as a scheduled sync into your vault or a ZIP download

## Try it

Open the [live demo](https://highlights-demo.mrlokans.work/) (read-only, public-domain books), or run the same demo locally:

```bash
docker run --rm -p 127.0.0.1:8080:8080 -e DEMO_MODE=true -e DEMO_USE_EMBEDDED=true \
  ghcr.io/mrlokans/highlights-manager:latest
```

Then open http://localhost:8080. Demo mode blocks writes; press Ctrl+C to stop it.

## Install

You need [Docker](https://docs.docker.com/get-docker/) with the Compose plugin (check with `docker compose version`).

**1. Create a directory for the database and the exported notes:**
```bash
mkdir -p highlights-manager/{data,vault}
cd highlights-manager
```

**2. Create `docker-compose.yml` in it:**
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
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```
To export straight into an existing Obsidian vault, replace `./vault` with the vault's path.

**3. Start it:**
```bash
docker compose up -d
```

**4. Check it:** `curl http://localhost:8080/health` should return HTTP 200, and http://localhost:8080 should show an empty library. Startup logs contain `record not found ... settings` lines on a fresh database; these are harmless. Stop it with `docker compose down`; your data stays in `./data`.

This setup has no login, so keep it bound to localhost. To expose it, follow [Self-hosting](docs/self-hosting.md) first.

## Import your highlights and export to Obsidian

1. Open http://localhost:8080/import and pick your source:
   - **Kindle:** connect it by USB and upload `documents/My Clippings.txt`.
   - **Apple Books:** upload the two `.sqlite` files from your Mac. The page's "Where are they?" link shows the folder.
   - **Readwise:** upload the CSV from Readwise's export page, or connect the Readwise API under **Connected sources**.
   - **Moon+ Reader:** connect Dropbox under **Connected sources** (needs [Dropbox app setup](docs/oauth2-setup.md)).

   Re-importing the same file only adds new highlights.
2. Your books appear in the **Library** at http://localhost:8080.
3. Export them:
   - **Export all** on the Library page gives a ZIP of Markdown notes, one per book. Each book's page also has its own download.
   - **Settings → Obsidian** syncs notes into the vault folder (`./vault` above) on a schedule.

## Next steps

| I want to… | Read |
|---|---|
| Add a login, run it in production, back it up, fix problems | [docs/self-hosting.md](docs/self-hosting.md) |
| Change ports, paths, sync schedule, auth or task settings | [docs/configuration.md](docs/configuration.md) |
| Script imports or queries over HTTP | [docs/api.md](docs/api.md) |
| Import from local files on the command line | [docs/cli.md](docs/cli.md) |
| Run from source, test or contribute | [docs/development.md](docs/development.md) |

## License

MIT
