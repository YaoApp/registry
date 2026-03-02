package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yaoapp/registry/models"
)

// List handles GET /v1/:type — list packages of a given type.
func (s *Server) List(c *gin.Context) {
	rawType := c.Param("type")
	singular, err := validateType(rawType)
	if err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	scope := c.Query("scope")
	q := c.Query("q")
	page, pageSize := bindPagination(c)

	result, err := models.ListPackages(s.DB, singular, scope, q, page, pageSize)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "list packages: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total":    result.Total,
		"page":     page,
		"pagesize": pageSize,
		"packages": result.Packages,
	})
}
