package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yaoapp/registry/models"
)

// Dependents handles GET /v1/:type/:scope/:name/dependents — reverse dependency lookup.
func (s *Server) Dependents(c *gin.Context) {
	singular, scope, name, ok := s.validateTypeAndScope(c)
	if !ok {
		return
	}

	dependents, err := models.GetDependents(s.DB, singular, scope, name)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]gin.H, 0, len(dependents))
	for _, d := range dependents {
		result = append(result, gin.H{
			"type": singularToPlural(d.Type), "scope": d.Scope,
			"name": d.Name, "version": d.Version,
		})
	}
	c.JSON(http.StatusOK, gin.H{"dependents": result})
}
