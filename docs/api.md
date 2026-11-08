# HTTP API

Examples use `http://localhost:8080`. With `AUTH_MODE=local`, API calls need an authenticated session or API token.

## Health & Status

```bash
# Health check (useful for monitoring)
curl http://localhost:8080/health
```

## Books

```bash
# List all books
curl http://localhost:8080/api/books

# Find a book by exact title and author (both required, case-sensitive)
curl "http://localhost:8080/api/books/search?title=Sapiens&author=Yuval%20Noah%20Harari"

# Get statistics
curl http://localhost:8080/api/books/stats

# Enrich book metadata
curl -X POST http://localhost:8080/api/books/123/enrich
```

## Imports

```bash
# Import Kindle clippings
curl -X POST http://localhost:8080/import/kindle \
  -F "clippings_file=@My Clippings.txt"
```

`My Clippings.txt` is in the `documents` folder of your Kindle when it's connected over USB.

## Tags

```bash
# List tags
curl http://localhost:8080/api/tags

# Create a tag (returns its id)
curl -X POST http://localhost:8080/api/tags \
  -H "Content-Type: application/json" \
  -d '{"name": "philosophy"}'

# Add an existing tag to an existing book
curl -X POST http://localhost:8080/api/books/123/tags \
  -H "Content-Type: application/json" \
  -d '{"tag_id": 456}'
```
