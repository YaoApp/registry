package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Version is the registry server version. Set at build time via ldflags.
var Version = "1.0.1"

// Info handles GET /v1/ — registry info.
func (s *Server) Info(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"name":    "yao-registry",
		"version": Version,
	})
}
