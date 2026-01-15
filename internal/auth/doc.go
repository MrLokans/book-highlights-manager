// Package auth provides authentication and authorization for the application.
//
// It supports two authentication modes:
//   - "none": No authentication required (default), all requests use a default user ID
//   - "local": Local user database with session cookies for web UI and Bearer tokens for API
//
// # Configuration
//
// Set AUTH_MODE environment variable to select the mode:
//
//	AUTH_MODE=none   # Default, no auth required
//	AUTH_MODE=local  # Requires user creation and login
//
// For local mode, additional configuration:
//
//	AUTH_SESSION_SECRET=<base64-32-bytes>  # Auto-generated if empty
//	AUTH_SESSION_LIFETIME=24h              # Session duration
//	AUTH_TOKEN_EXPIRY=720h                 # API token expiry (30 days default)
//	AUTH_BCRYPT_COST=12                    # bcrypt cost factor
//	AUTH_SECURE_COOKIES=true               # HTTPS-only cookies
//
// # Usage
//
// Initialize authentication in entrypoint:
//
//	authService := auth.NewService(userRepo, cfg.Auth)
//	authMiddleware := auth.NewMiddleware(authService, cfg.Auth)
//	router.Use(authMiddleware.Handler())
//
// Extract user in handlers:
//
//	userID := auth.GetUserID(c)  // Returns DefaultUserID in "none" mode
package auth
