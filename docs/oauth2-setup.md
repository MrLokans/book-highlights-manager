# OAuth2 Setup Guide

This guide explains how to set up OAuth2 authentication for cloud storage integrations like Dropbox.

## Overview

The Assistant application uses OAuth2 to securely access cloud storage services. This allows features like:
- Importing MoonReader highlights from Dropbox backups
- Future integrations with Google Drive, OneDrive, etc.

## Dropbox Setup

### Step 1: Create a Dropbox App

1. Go to [Dropbox Developer Console](https://www.dropbox.com/developers/apps)
2. Click **Create app**
3. Choose settings:
   - **API**: Scoped access
   - **Access type**: Choose based on your needs:
     - **App folder**: Only access files in a dedicated app folder (more secure, limited access)
     - **Full Dropbox**: Access all files in your Dropbox (needed if backups are outside app folder)
   - **Name**: Choose a descriptive name (e.g., "MoonReader Highlights Sync")
4. Click **Create app**

### Step 2: Configure OAuth2 Settings

In your app's settings page:

1. Under **OAuth 2**, find **Redirect URIs**
2. Add: `http://localhost:8089/callback`
3. Note your **App key** (you'll need this)

### Step 3: Set Environment Variables

Add to your shell profile or `.env-local` file:

```bash
export DROPBOX_APP_KEY=your_app_key_here
```

### Step 4: Run Authentication

Run the authentication command:

```bash
./assistant dropbox-auth
```

This will:
1. Open a URL in your browser
2. Ask you to authorize the application
3. Receive the authorization callback
4. Securely store encrypted tokens in your database

#### Options

```bash
# Use a different port for the callback server
./assistant dropbox-auth -port 8090

# Manual flow (copy/paste code instead of callback)
./assistant dropbox-auth -manual

# Don't save tokens (print them instead)
./assistant dropbox-auth -no-save

# Use a specific database path
./assistant dropbox-auth -db /path/to/database.db
```

### Step 5: Verify Setup

Test that everything works:

```bash
# List MoonReader backups in Dropbox
./assistant moonreader-dropbox -list

# Full sync
./assistant moonreader-dropbox
```

## Token Storage

Tokens are stored in an encrypted SQLite database:

- **Default location**: `./highlights-manager.db`
- **Encryption**: AES-256-GCM
- **Key location**: `~/.assistant-token-key` (auto-generated if missing)

### Custom Encryption Key

For production deployments, you can provide your own encryption key:

```bash
# Generate a key
openssl rand -base64 32

# Set via environment variable
export TOKEN_ENCRYPTION_KEY=your_base64_key_here
```

## Token Refresh

The application automatically refreshes tokens:

- **On-demand**: Before each API call, checks if token is expiring soon
- **Background**: Optional scheduler refreshes tokens proactively

### Configuration

Environment variables for the background refresh scheduler:

```bash
# Enable/disable background refresh (default: true)
export OAUTH2_REFRESH_ENABLED=true

# How often to check for expiring tokens (default: 30m)
export OAUTH2_CHECK_INTERVAL=30m

# Refresh tokens this long before they expire (default: 15m)
export OAUTH2_REFRESH_MARGIN=15m
```

## Troubleshooting

### "Port 8089 is not available"

Another process is using port 8089. Either:
- Stop the conflicting process, or
- Use a different port: `./assistant dropbox-auth -port 8090`
- Remember to add the new redirect URI in Dropbox app settings

### "State mismatch: possible CSRF attack"

This usually means:
- You completed an old authorization flow
- Try again with a fresh `dropbox-auth` command

### "No backup files found"

Check your Dropbox path setting:

```bash
# List all files in the default path
./assistant moonreader-dropbox -list-all

# For App Folder apps, use empty path
./assistant moonreader-dropbox -list-all -dropbox-path=""
```

App Folder apps see their folder as root, so use `-dropbox-path=""` instead of the full path.

### "Token expired and no refresh token available"

The initial authorization didn't request offline access. Re-run:

```bash
./assistant dropbox-auth
```

This will request a new token with refresh capability.

### "Failed to refresh token"

Ensure `DROPBOX_APP_KEY` is set when running commands that need token refresh:

```bash
export DROPBOX_APP_KEY=your_app_key
./assistant moonreader-dropbox
```

### Viewing Stored Tokens

Tokens are encrypted, but you can see metadata:

```bash
sqlite3 highlights-manager.db "SELECT provider, account_id, expires_at FROM oauth_tokens"
```

### Removing Stored Tokens

To remove stored tokens and start fresh:

```bash
sqlite3 highlights-manager.db "DELETE FROM oauth_tokens WHERE provider='dropbox'"
```

Then re-run `dropbox-auth`.

## Security Best Practices

1. **Never share your App Key** - Keep it in environment variables, not source code
2. **Protect the encryption key** - The `~/.assistant-token-key` file contains your encryption key
3. **Use App Folder access** when possible - Limits what the app can access
4. **Review app permissions** periodically at https://www.dropbox.com/account/connected_apps

## Adding New Providers

The OAuth2 system is designed to be extensible. To add support for a new provider (e.g., Google Drive):

1. Create a new provider in `internal/oauth2/providers/`
2. Implement the `oauth2.Provider` interface
3. Register the provider in the registry
4. Create a new storage client in `internal/storage/providers/`

See `internal/oauth2/providers/dropbox.go` for a reference implementation.
