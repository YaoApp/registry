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

	if c.Request.ContentLength > s.Config.MaxSizeBytes() {
		jsonError(c, http.StatusRequestEntityTooLarge, "package too large")
		return
	}

	limitReader := http.MaxBytesReader(c.Writer, c.Request.Body, s.Config.MaxSizeBytes())
	zipData, err := io.ReadAll(limitReader)
	if err != nil {
		jsonError(c, http.StatusRequestEntityTooLarge, "package too large or read error")
		return
	}

	pkgYao, err := pack.ExtractPkgYao(zipData)
	if err != nil {
		jsonError(c, http.StatusBadRequest, "extract pkg.yao: "+err.Error())
		return
	}

	if err := pack.ValidatePkgYao(pkgYao, singular, scope, name, version); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	readme, _ := pack.ExtractReadme(zipData)
	digest := storage.ComputeDigest(zipData)

	goos, arch, variant := "", "", ""
	if pkgYao.Platform != nil {
		goos = pkgYao.Platform.OS
		arch = pkgYao.Platform.Arch
		variant = pkgYao.Platform.Variant
	}

	filePath, err := storage.Store(s.Config.DataPath, singularToPlural(singular),
		scope, name, version, goos, arch, variant, zipData)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "store file: "+err.Error())
		return
	}

	metadataMap := map[string]interface{}{}
	if pkgYao.Metadata != nil {
		metadataMap = pkgYao.Metadata
	}
	if pkgYao.Engines != nil {
		metadataMap["engines"] = pkgYao.Engines
	}
	metadataJSON, _ := json.Marshal(metadataMap)

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

	// Step 1: upsert package
	pkgID, err := models.UpsertPackage(s.DB, &models.Package{
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
		DistTags:    "{}",
	})
	if err != nil {
		storage.Delete(s.Config.DataPath, filePath)
		jsonError(c, http.StatusInternalServerError, "upsert package: "+err.Error())
		return
	}

	// Step 2: insert version
	verID, err := models.InsertVersion(s.DB, &models.Version{
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
		storage.Delete(s.Config.DataPath, filePath)
		jsonError(c, http.StatusConflict, "version already exists")
		return
	}

	// Step 3: insert dependencies (rollback = delete version + deps + file)
	if len(pkgYao.Dependencies) > 0 {
		deps := make([]models.Dependency, len(pkgYao.Dependencies))
		for i, d := range pkgYao.Dependencies {
			deps[i] = models.Dependency{
				DepType: d.Type, DepScope: d.Scope,
				DepName: d.Name, DepVersion: d.Version,
			}
		}
		if err := models.InsertDependencies(s.DB, verID, deps); err != nil {
			models.DeleteDependenciesByVersion(s.DB, verID)
			models.DeleteVersion(s.DB, verID)
			storage.Delete(s.Config.DataPath, filePath)
			jsonError(c, http.StatusInternalServerError, "insert deps: "+err.Error())
			return
		}
	}

	// Step 4: update dist_tags
	pkg, err := models.GetPackageByID(s.DB, pkgID)
	if err != nil {
		pkg = &models.Package{DistTags: "{}"}
	}
	var distTags map[string]string
	json.Unmarshal([]byte(pkg.DistTags), &distTags)
	if distTags == nil {
		distTags = map[string]string{}
	}

	if !containsHyphen(version) {
		distTags["latest"] = version
	} else if _, ok := distTags["latest"]; !ok {
		distTags["latest"] = version
	}

	distTagsJSON, _ := json.Marshal(distTags)
	if err := models.UpdateDistTags(s.DB, pkgID, string(distTagsJSON)); err != nil {
		jsonError(c, http.StatusInternalServerError, "update dist_tags: "+err.Error())
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
