package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yaoapp/registry/models"
	"github.com/yaoapp/registry/pack"
	"github.com/yaoapp/registry/storage"
)

// Push handles PUT /v1/:type/:scope/:name/:version — upload a .yao.zip package.
func (s *Server) Push(c *gin.Context) {
	singular, scope, name, ok := s.validateTypeAndScope(c)
	if !ok {
		return
	}
	version := c.Param("version")

	// Early size check via Content-Length header
	if c.Request.ContentLength > s.Config.MaxSizeBytes() {
		jsonError(c, http.StatusRequestEntityTooLarge, "package too large")
		return
	}

	// Limit body reader to prevent abuse
	limitReader := http.MaxBytesReader(c.Writer, c.Request.Body, s.Config.MaxSizeBytes())
	zipData, err := io.ReadAll(limitReader)
	if err != nil {
		jsonError(c, http.StatusRequestEntityTooLarge, "package too large or read error")
		return
	}

	// Extract and validate pkg.yao
	pkgYao, err := pack.ExtractPkgYao(zipData)
	if err != nil {
		jsonError(c, http.StatusBadRequest, "extract pkg.yao: "+err.Error())
		return
	}

	if err := pack.ValidatePkgYao(pkgYao, singular, scope, name, version); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	// Extract README
	readme, _ := pack.ExtractReadme(zipData)

	// Compute digest
	digest := storage.ComputeDigest(zipData)

	// Determine platform fields for release packages
	goos, arch, variant := "", "", ""
	if pkgYao.Platform != nil {
		goos = pkgYao.Platform.OS
		arch = pkgYao.Platform.Arch
		variant = pkgYao.Platform.Variant
	}

	// Store the file
	filePath, err := storage.Store(s.Config.DataPath, singularToPlural(singular),
		scope, name, version, goos, arch, variant, zipData)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "store file: "+err.Error())
		return
	}

	// Build metadata JSON from pkg.yao extra fields
	metadataMap := map[string]interface{}{}
	if pkgYao.Metadata != nil {
		metadataMap = pkgYao.Metadata
	}
	if pkgYao.Engines != nil {
		metadataMap["engines"] = pkgYao.Engines
	}
	metadataJSON, _ := json.Marshal(metadataMap)

	// Database transaction
	tx, err := s.DB.Begin()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "begin tx: "+err.Error())
		return
	}
	defer tx.Rollback()

	// Prepare package metadata
	keywordsJSON, _ := json.Marshal(pkgYao.Keywords)
	if pkgYao.Keywords == nil {
		keywordsJSON = []byte("[]")
	}
	authorJSON, _ := json.Marshal(pkgYao.Author)
	if pkgYao.Author == nil {
		authorJSON = []byte("{}")
	}
	maintainersJSON, _ := json.Marshal(pkgYao.Maintainers)
	if pkgYao.Maintainers == nil {
		maintainersJSON = []byte("[]")
	}
	repoJSON, _ := json.Marshal(pkgYao.Repository)
	if pkgYao.Repository == nil {
		repoJSON = []byte("{}")
	}
	bugsJSON, _ := json.Marshal(pkgYao.Bugs)
	if pkgYao.Bugs == nil {
		bugsJSON = []byte("{}")
	}

	// Upsert package
	pkgID, err := models.UpsertPackage(tx, &models.Package{
		Type:        singular,
		Scope:       scope,
		Name:        name,
		Description: pkgYao.Description,
		Keywords:    string(keywordsJSON),
		Icon:        pkgYao.Icon,
		License:     pkgYao.License,
		Author:      string(authorJSON),
		Maintainers: string(maintainersJSON),
		Homepage:    pkgYao.Homepage,
		Repository:  string(repoJSON),
		Bugs:        string(bugsJSON),
		Readme:      readme,
		DistTags:    "{}", // will be updated below
	})
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "upsert package: "+err.Error())
		return
	}

	// Insert version
	verID, err := models.InsertVersion(tx, &models.Version{
		PackageID: pkgID,
		Version:   version,
		OS:        goos,
		Arch:      arch,
		Variant:   variant,
		Digest:    digest,
		Size:      int64(len(zipData)),
		Metadata:  string(metadataJSON),
		FilePath:  filePath,
	})
	if err != nil {
		// Clean up stored file on version insert failure (likely duplicate)
		storage.Delete(s.Config.DataPath, filePath)
		jsonError(c, http.StatusConflict, "version already exists")
		return
	}

	// Insert dependencies
	if len(pkgYao.Dependencies) > 0 {
		deps := make([]models.Dependency, len(pkgYao.Dependencies))
		for i, d := range pkgYao.Dependencies {
			deps[i] = models.Dependency{
				DepType: d.Type, DepScope: d.Scope,
				DepName: d.Name, DepVersion: d.Version,
			}
		}
		if err := models.InsertDependencies(tx, verID, deps); err != nil {
			storage.Delete(s.Config.DataPath, filePath)
			jsonError(c, http.StatusInternalServerError, "insert deps: "+err.Error())
			return
		}
	}

	// Update dist_tags: set "latest" if this is a non-prerelease version
	pkg, err := models.GetPackageByID(s.DB, pkgID)
	if err != nil {
		// Fall back: use empty dist_tags
		pkg = &models.Package{DistTags: "{}"}
	}
	var distTags map[string]string
	json.Unmarshal([]byte(pkg.DistTags), &distTags)
	if distTags == nil {
		distTags = map[string]string{}
	}

	// Set latest for non-prerelease versions (no hyphen in version)
	if !containsHyphen(version) {
		distTags["latest"] = version
	} else if _, ok := distTags["latest"]; !ok {
		// First ever push is a prerelease — still set latest
		distTags["latest"] = version
	}

	distTagsJSON, _ := json.Marshal(distTags)
	if err := models.UpdateDistTags(tx, pkgID, string(distTagsJSON)); err != nil {
		jsonError(c, http.StatusInternalServerError, "update dist_tags: "+err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		storage.Delete(s.Config.DataPath, filePath)
		jsonError(c, http.StatusInternalServerError, "commit: "+err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"type": singularToPlural(singular), "scope": scope,
		"name": name, "version": version, "digest": digest,
	})
}

func containsHyphen(s string) bool {
	for _, c := range s {
		if c == '-' {
			return true
		}
	}
	return false
}
