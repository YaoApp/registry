package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yaoapp/registry/models"
)

// Search handles GET /v1/search — cross-type package search.
func (s *Server) Search(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		jsonError(c, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	pkgType := c.Query("type")
	if pkgType != "" {
		if singular, err := validateType(pkgType); err == nil {
			pkgType = singular
		} else {
			jsonError(c, http.StatusBadRequest, err.Error())
			return
		}
	}

	page, pageSize := bindPagination(c)
	result, err := models.SearchPackages(s.DB, q, pkgType, page, pageSize)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "search: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total":    result.Total,
		"page":     page,
		"pagesize": pageSize,
		"packages": result.Packages,
	})
}
