# CLI imports

Most imports are easier from the web UI (**Import** page). The CLI is for scripting or importing straight from local files on the machine with the source checkout.

Run commands from the repository root (needs Go, see [development.md](development.md)). They write to `./highlights-manager.db` (Moon+ commands: `./moonreader.db`) unless you pass `-db <path>`; `DATABASE_PATH` is not used here. Add `-h` to any command for its flags.

```bash
# Apple Books import (macOS only, reads your Apple Books library)
go run main.go applebooks-import

# Kindle import from file
go run main.go kindle-import -file "/path/to/My Clippings.txt"

# Moon+ Reader from a local folder of Moon+ backup files
go run main.go moonreader-sync -backup-dir /path/to/MoonReader/Backup

# Moon+ Reader from Dropbox
go run main.go moonreader-dropbox
```
