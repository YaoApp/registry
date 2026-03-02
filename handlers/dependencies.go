package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yaoapp/registry/models"
)

// Dependencies handles GET /v1/:type/:scope/:name/:version/dependencies.
// Supports ?recursive=true for full dependency tree resolution.
func (s *Server) Dependencies(c *gin.Context) {
	singular, scope, name, ok := s.validateTypeAndScope(c)
	if !ok {
		return
	}
	version := c.Param("version")

	pkg, err := models.GetPackage(s.DB, singular, scope, name)
	if err == sql.ErrNoRows {
		jsonError(c, http.StatusNotFound, "package not found")
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	ver, err := models.GetVersion(s.DB, pkg.ID, version, "", "", "")
	if err == sql.ErrNoRows {
		jsonError(c, http.StatusNotFound, "version not found")
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	if c.Query("recursive") == "true" {
		tree, err := models.ResolveDependencyTree(s.DB, ver.ID)
		if err != nil {
			jsonError(c, http.StatusInternalServerError, "resolve tree: "+err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"dependencies": tree})
		return
	}

	deps, err := models.GetDependencies(s.DB, ver.ID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]gin.H, 0, len(deps))
	for _, d := range deps {
		result = append(result, gin.H{
			"type": d.DepType, "scope": d.DepScope,
			"name": d.DepName, "version": d.DepVersion,
		})
	}
	c.JSON(http.StatusOK, gin.H{"dependencies": result})
}
