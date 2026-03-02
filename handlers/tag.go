package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yaoapp/registry/models"
)

// TagSet handles PUT /v1/:type/:scope/:name/tags/:tag — set a dist-tag.
func (s *Server) TagSet(c *gin.Context) {
	singular, scope, name, ok := s.validateTypeAndScope(c)
	if !ok {
		return
	}
	tag := c.Param("tag")

	var body struct {
		Version string `json:"version" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonError(c, http.StatusBadRequest, "body must contain {\"version\": \"x.y.z\"}")
		return
	}

	pkg, err := models.GetPackage(s.DB, singular, scope, name)
	if err == sql.ErrNoRows {
		jsonError(c, http.StatusNotFound, "package not found")
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Verify the target version exists
	_, err = models.GetVersion(s.DB, pkg.ID, body.Version, "", "", "")
	if err == sql.ErrNoRows {
		jsonError(c, http.StatusNotFound, "version "+body.Version+" not found")
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	var distTags map[string]string
	json.Unmarshal([]byte(pkg.DistTags), &distTags)
	if distTags == nil {
		distTags = map[string]string{}
	}

	distTags[tag] = body.Version
	distTagsJSON, _ := json.Marshal(distTags)

	tx, err := s.DB.Begin()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()

	if err := models.UpdateDistTags(tx, pkg.ID, string(distTagsJSON)); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	tx.Commit()

	c.JSON(http.StatusOK, gin.H{"tag": tag, "version": body.Version})
}

// TagDelete handles DELETE /v1/:type/:scope/:name/tags/:tag — remove a dist-tag.
func (s *Server) TagDelete(c *gin.Context) {
	singular, scope, name, ok := s.validateTypeAndScope(c)
	if !ok {
		return
	}
	tag := c.Param("tag")

	if tag == "latest" {
		jsonError(c, http.StatusBadRequest, "cannot delete the 'latest' tag")
		return
	}

	pkg, err := models.GetPackage(s.DB, singular, scope, name)
	if err == sql.ErrNoRows {
		jsonError(c, http.StatusNotFound, "package not found")
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	var distTags map[string]string
	json.Unmarshal([]byte(pkg.DistTags), &distTags)
	if distTags == nil {
		distTags = map[string]string{}
	}

	if _, exists := distTags[tag]; !exists {
		jsonError(c, http.StatusNotFound, "tag not found")
		return
	}

	delete(distTags, tag)
	distTagsJSON, _ := json.Marshal(distTags)

	tx, err := s.DB.Begin()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()

	if err := models.UpdateDistTags(tx, pkg.ID, string(distTagsJSON)); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	tx.Commit()

	c.JSON(http.StatusOK, gin.H{"deleted": tag})
}
