package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yaoapp/registry/models"
)

// VersionDetail handles GET /v1/:type/:scope/:name/:version — single version metadata.
func (s *Server) VersionDetail(c *gin.Context) {
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

	// For release type, allow platform filtering
	if singular == "release" {
		os := c.Query("os")
		arch := c.Query("arch")
		variant := c.Query("variant")

		if os != "" || arch != "" || variant != "" {
			versions, err := models.ListVersionsByPlatform(s.DB, pkg.ID, version, os, arch, variant)
			if err != nil {
				jsonError(c, http.StatusInternalServerError, err.Error())
				return
			}
			if len(versions) == 0 {
				jsonError(c, http.StatusNotFound, "no matching platform artifacts")
				return
			}

			artifacts := make([]gin.H, 0, len(versions))
			for _, v := range versions {
				var metadata map[string]interface{}
				json.Unmarshal([]byte(v.Metadata), &metadata)
				artifacts = append(artifacts, gin.H{
					"os": v.OS, "arch": v.Arch, "variant": v.Variant,
					"digest": v.Digest, "size": v.Size,
					"metadata": metadata, "created_at": v.CreatedAt,
				})
			}
			c.JSON(http.StatusOK, gin.H{
				"type": singularToPlural(singular), "scope": scope,
				"name": name, "version": version, "artifacts": artifacts,
			})
			return
		}

		// List all platform artifacts for this version
		versions, err := models.ListVersionsByPlatform(s.DB, pkg.ID, version, "", "", "")
		if err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		if len(versions) == 0 {
			jsonError(c, http.StatusNotFound, "version not found")
			return
		}

		artifacts := make([]gin.H, 0, len(versions))
		for _, v := range versions {
			var metadata map[string]interface{}
			json.Unmarshal([]byte(v.Metadata), &metadata)
			artifacts = append(artifacts, gin.H{
				"os": v.OS, "arch": v.Arch, "variant": v.Variant,
				"digest": v.Digest, "size": v.Size,
				"metadata": metadata, "created_at": v.CreatedAt,
			})
		}
		c.JSON(http.StatusOK, gin.H{
			"type": singularToPlural(singular), "scope": scope,
			"name": name, "version": version, "artifacts": artifacts,
		})
		return
	}

	// Non-release: single version lookup
	ver, err := models.GetVersion(s.DB, pkg.ID, version, "", "", "")
	if err == sql.ErrNoRows {
		jsonError(c, http.StatusNotFound, "version not found")
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	deps, _ := models.GetDependencies(s.DB, ver.ID)
	depList := make([]gin.H, 0, len(deps))
	for _, d := range deps {
		depList = append(depList, gin.H{
			"type": d.DepType, "scope": d.DepScope,
			"name": d.DepName, "version": d.DepVersion,
		})
	}

	var metadata map[string]interface{}
	json.Unmarshal([]byte(ver.Metadata), &metadata)

	c.JSON(http.StatusOK, gin.H{
		"type": singularToPlural(singular), "scope": scope,
		"name": name, "version": ver.Version,
		"digest": ver.Digest, "size": ver.Size,
		"dependencies": depList, "metadata": metadata,
		"created_at": ver.CreatedAt,
	})
}
