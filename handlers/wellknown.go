package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// WellKnown handles GET /.well-known/yao-registry — discovery endpoint.
func (s *Server) WellKnown(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"registry": gin.H{
			"version": "1",
			"api":     "/v1",
		},
		"types": []string{"releases", "robots", "assistants", "mcps"},
	})
}
