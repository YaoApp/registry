// Package handlers provides HTTP handlers for all registry API endpoints.
package handlers

import (
	"database/sql"

	"github.com/gin-gonic/gin"
	"github.com/yaoapp/registry/auth"
	"github.com/yaoapp/registry/config"
	"github.com/yaoapp/registry/middleware"
)

// Server holds shared dependencies for all handlers.
type Server struct {
	DB       *sql.DB
	Config   *config.Config
	AuthFile *auth.AuthFile
}

// SetupRoutes registers all API routes on the given Gin engine.
func (s *Server) SetupRoutes(r *gin.Engine) {
	r.GET("/.well-known/yao-registry", s.WellKnown)

	v1 := r.Group("/v1")
	{
		v1.GET("/", s.Info)
		v1.GET("/search", s.Search)

		v1.GET("/:type", s.List)
		v1.GET("/:type/:scope/:name", s.Packument)
		v1.GET("/:type/:scope/:name/dependents", s.Dependents)

		v1.GET("/:type/:scope/:name/:version", s.VersionDetail)
		v1.GET("/:type/:scope/:name/:version/pull", s.Pull)
		v1.GET("/:type/:scope/:name/:version/dependencies", s.Dependencies)

		authGroup := v1.Group("", middleware.BasicAuth(s.AuthFile))
		{
			authGroup.PUT("/:type/:scope/:name/tags/:tag", s.TagSet)
			authGroup.DELETE("/:type/:scope/:name/tags/:tag", s.TagDelete)
			authGroup.PUT("/:type/:scope/:name/:version", s.Push)
			authGroup.DELETE("/:type/:scope/:name/:version", s.Delete)
		}
	}
}
