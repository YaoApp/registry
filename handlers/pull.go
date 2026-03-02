package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yaoapp/registry/models"
	"github.com/yaoapp/registry/storage"
)

// Pull handles GET /v1/:type/:scope/:name/:version/pull — download a .yao.zip.
// The :version can be a semver string or a dist-tag name.
func (s *Server) Pull(c *gin.Context) {
	singular, scope, name, ok := s.validateTypeAndScope(c)
	if !ok {
		return
	}
	versionOrTag := c.Param("version")

	pkg, err := models.GetPackage(s.DB, singular, scope, name)
	if err == sql.ErrNoRows {
		jsonError(c, http.StatusNotFound, "package not found")
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Check if versionOrTag is a dist-tag
	version := versionOrTag
	var distTags map[string]string
	json.Unmarshal([]byte(pkg.DistTags), &distTags)
	if resolved, ok := distTags[versionOrTag]; ok {
		version = resolved
	}

	// Platform query params for release type
	goos := c.Query("os")
	arch := c.Query("arch")
	variant := c.Query("variant")

	ver, err := models.GetVersion(s.DB, pkg.ID, version, goos, arch, variant)
	if err == sql.ErrNoRows {
		jsonError(c, http.StatusNotFound, "version not found")
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	rc, size, err := storage.Load(s.Config.DataPath, ver.FilePath)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "load file: "+err.Error())
		return
	}
	defer rc.Close()

	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s.yao.zip"`, name, version))
	c.Header("X-Digest", ver.Digest)
	c.Header("Content-Length", fmt.Sprintf("%d", size))
	c.DataFromReader(http.StatusOK, size, "application/zip", rc, nil)
}
