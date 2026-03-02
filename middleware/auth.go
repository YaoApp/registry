// Package middleware provides Gin middleware for the registry server.
package middleware

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yaoapp/registry/auth"
)

// BasicAuth returns a Gin middleware that validates Basic Authentication
// against the provided AuthFile. It responds with 401 on failure.
func BasicAuth(af *auth.AuthFile) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			unauthorized(c, "missing Authorization header")
			return
		}

		if !strings.HasPrefix(header, "Basic ") {
			unauthorized(c, "invalid Authorization scheme")
			return
		}

		decoded, err := base64.StdEncoding.DecodeString(header[6:])
		if err != nil {
			unauthorized(c, "malformed base64 credentials")
			return
		}

		parts := strings.SplitN(string(decoded), ":", 2)
		if len(parts) != 2 {
			unauthorized(c, "malformed credentials")
			return
		}

		username, password := parts[0], parts[1]
		if !af.Verify(username, password) {
			unauthorized(c, "invalid username or password")
			return
		}

		c.Set("username", username)
		c.Next()
	}
}

func unauthorized(c *gin.Context, msg string) {
	c.Header("WWW-Authenticate", `Basic realm="yao-registry"`)
	c.JSON(http.StatusUnauthorized, gin.H{"error": msg})
	c.Abort()
}
