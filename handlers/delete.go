package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yaoapp/registry/models"
	"github.com/yaoapp/registry/storage"
)

// Delete handles DELETE /v1/:type/:scope/:name/:version — remove a version.
func (s *Server) Delete(c *gin.Context) {
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

	// Step 1: delete dependencies
	models.DeleteDependenciesByVersion(s.DB, ver.ID)

	// Step 2: delete version
	if err := models.DeleteVersion(s.DB, ver.ID); err != nil {
		jsonError(c, http.StatusInternalServerError, "delete version: "+err.Error())
		return
	}

	// Step 3: update dist_tags if the deleted version was tagged
	var distTags map[string]string
	json.Unmarshal([]byte(pkg.DistTags), &distTags)
	if distTags == nil {
		distTags = map[string]string{}
	}

	tagsChanged := false
	for tag, tagVer := range distTags {
		if tagVer == version {
			if tag == "latest" {
				newLatest, err := models.GetLatestNonPrerelease(s.DB, pkg.ID)
				if err == nil && newLatest != "" {
					distTags["latest"] = newLatest
				} else {
					delete(distTags, "latest")
				}
			} else {
				delete(distTags, tag)
			}
			tagsChanged = true
		}
	}

	if tagsChanged {
		distTagsJSON, _ := json.Marshal(distTags)
		models.UpdateDistTags(s.DB, pkg.ID, string(distTagsJSON))
	}

	// Best-effort file removal after DB cleanup
	storage.Delete(s.Config.DataPath, ver.FilePath)

	c.JSON(http.StatusOK, gin.H{
		"deleted": version,
		"type":    singularToPlural(singular),
		"scope":   scope,
		"name":    name,
	})
}
