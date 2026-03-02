package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yaoapp/registry/models"
)

// Packument handles GET /v1/:type/:scope/:name — full package metadata.
// Supports abbreviated metadata via Accept: application/vnd.yao.abbreviated+json.
func (s *Server) Packument(c *gin.Context) {
	singular, scope, name, ok := s.validateTypeAndScope(c)
	if !ok {
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

	versions, err := models.ListVersions(s.DB, pkg.ID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	abbreviated := strings.Contains(c.GetHeader("Accept"), "abbreviated")

	var distTags map[string]string
	json.Unmarshal([]byte(pkg.DistTags), &distTags)
	if distTags == nil {
		distTags = map[string]string{}
	}

	versionsMap := make(map[string]interface{})
	for _, v := range versions {
		deps, _ := models.GetDependencies(s.DB, v.ID)
		depList := make([]gin.H, 0, len(deps))
		for _, d := range deps {
			depList = append(depList, gin.H{
				"type": d.DepType, "scope": d.DepScope,
				"name": d.DepName, "version": d.DepVersion,
			})
		}

		var metadata map[string]interface{}
		json.Unmarshal([]byte(v.Metadata), &metadata)
		if metadata == nil {
			metadata = map[string]interface{}{}
		}

		// Extract engines from metadata for top-level exposure
		engines, _ := metadata["engines"].(map[string]interface{})

		if abbreviated {
			entry := gin.H{
				"version":      v.Version,
				"digest":       v.Digest,
				"dependencies": depList,
			}
			if engines != nil {
				entry["engines"] = engines
			}
			if v.OS != "" || v.Arch != "" {
				entry["os"] = v.OS
				entry["arch"] = v.Arch
				entry["variant"] = v.Variant
			}
			versionsMap[versionKey(v)] = entry
		} else {
			entry := gin.H{
				"version":      v.Version,
				"digest":       v.Digest,
				"size":         v.Size,
				"dependencies": depList,
				"metadata":     metadata,
				"created_at":   v.CreatedAt,
			}
			if engines != nil {
				entry["engines"] = engines
			}
			if v.OS != "" || v.Arch != "" {
				entry["os"] = v.OS
				entry["arch"] = v.Arch
				entry["variant"] = v.Variant
			}
			versionsMap[versionKey(v)] = entry
		}
	}

	var keywords []string
	json.Unmarshal([]byte(pkg.Keywords), &keywords)

	result := gin.H{
		"type":        singularToPlural(pkg.Type),
		"scope":       pkg.Scope,
		"name":        pkg.Name,
		"description": pkg.Description,
		"keywords":    keywords,
		"dist_tags":   distTags,
		"versions":    versionsMap,
		"created_at":  pkg.CreatedAt,
		"updated_at":  pkg.UpdatedAt,
	}

	if !abbreviated {
		result["license"] = pkg.License
		result["homepage"] = pkg.Homepage
		result["readme"] = pkg.Readme

		var author, repository, bugs interface{}
		json.Unmarshal([]byte(pkg.Author), &author)
		json.Unmarshal([]byte(pkg.Repository), &repository)
		json.Unmarshal([]byte(pkg.Bugs), &bugs)
		var maintainers interface{}
		json.Unmarshal([]byte(pkg.Maintainers), &maintainers)

		result["author"] = author
		result["maintainers"] = maintainers
		result["repository"] = repository
		result["bugs"] = bugs
	}

	c.JSON(http.StatusOK, result)
}

func versionKey(v *models.Version) string {
	key := v.Version
	if v.OS != "" {
		key += "-" + v.OS
		if v.Arch != "" {
			key += "-" + v.Arch
		}
		if v.Variant != "" {
			key += "-" + v.Variant
		}
	}
	return key
}
