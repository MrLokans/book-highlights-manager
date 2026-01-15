package auth

import (
	"bufio"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// sessionResponseWriter wraps http.ResponseWriter to intercept WriteHeader
// and write session cookies before headers are sent.
type sessionResponseWriter struct {
	gin.ResponseWriter
	sm            *SessionManager
	request       *http.Request
	wroteHeader   bool
	cookieWritten bool
}

func (w *sessionResponseWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.wroteHeader = true
		w.writeSessionCookie()
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *sessionResponseWriter) WriteHeaderNow() {
	if !w.wroteHeader {
		w.wroteHeader = true
		w.writeSessionCookie()
	}
	w.ResponseWriter.WriteHeaderNow()
}

func (w *sessionResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.wroteHeader = true
		w.writeSessionCookie()
	}
	return w.ResponseWriter.Write(b)
}

func (w *sessionResponseWriter) writeSessionCookie() {
	if w.cookieWritten {
		return
	}
	w.cookieWritten = true

	ctx := w.request.Context()
	switch w.sm.Status(ctx) {
	case 1: // Modified
		token, expiry, err := w.sm.Commit(ctx)
		if err != nil {
			return
		}
		w.sm.WriteSessionCookie(ctx, w.ResponseWriter, token, expiry)
	case 2: // Destroyed
		w.sm.WriteSessionCookie(ctx, w.ResponseWriter, "", time.Time{})
	}
}

// Implement http.Hijacker for WebSocket support
func (w *sessionResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.ResponseWriter.Hijack()
}

// SessionLoadSave returns a Gin middleware that wraps the session manager's
// LoadAndSave functionality. This must be used before any session operations.
func (sm *SessionManager) SessionLoadSave() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Load session data into request context
		var token string
		cookie, err := c.Request.Cookie(sm.Cookie.Name)
		if err == nil {
			token = cookie.Value
		}

		ctx, err := sm.Load(c.Request.Context(), token)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Request = c.Request.WithContext(ctx)

		// Wrap the response writer to intercept WriteHeader
		srw := &sessionResponseWriter{
			ResponseWriter: c.Writer,
			sm:             sm,
			request:        c.Request,
		}
		c.Writer = srw

		// Process the request
		c.Next()

		// Ensure session cookie is written even if no response body
		if !srw.wroteHeader {
			srw.writeSessionCookie()
		}
	}
}
