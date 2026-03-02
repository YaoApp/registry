package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var validTypes = map[string]string{
	"releases":   "release",
	"robots":     "robot",
	"assistants": "assistant",
	"mcps":       "mcp",
}

// validateType checks that the URL type parameter is one of the four allowed
// plural forms and returns the singular form for database storage.
func validateType(t string) (string, error) {
	if singular, ok := validTypes[t]; ok {
		return singular, nil
	}
	return "", fmt.Errorf("invalid type %q: must be one of releases, robots, assistants, mcps", t)
}

// singularToPlural converts a singular DB type to its plural URL form.
func singularToPlural(t string) string {
	for plural, singular := range validTypes {
		if singular == t {
			return plural
		}
	}
	return t
}

// parseScope validates that the scope starts with '@'.
func parseScope(raw string) (string, error) {
	if !strings.HasPrefix(raw, "@") {
		return "", fmt.Errorf("scope must start with @, got %q", raw)
	}
	return raw, nil
}

// jsonError responds with a JSON error message.
func jsonError(c *gin.Context, code int, msg string) {
	c.JSON(code, gin.H{"error": msg})
}

// bindPagination reads page and pagesize query params with defaults.
func bindPagination(c *gin.Context) (page, pageSize int) {
	page = 1
	pageSize = 20
	if v := c.Query("page"); v != "" {
		fmt.Sscanf(v, "%d", &page)
	}
	if v := c.Query("pagesize"); v != "" {
		fmt.Sscanf(v, "%d", &pageSize)
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return
}

// validateTypeAndScope is a convenience helper used by most handlers.
func (s *Server) validateTypeAndScope(c *gin.Context) (typeSingular, scope, name string, ok bool) {
	rawType := c.Param("type")
	singular, err := validateType(rawType)
	if err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return "", "", "", false
	}

	rawScope := c.Param("scope")
	scope, err = parseScope(rawScope)
	if err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return "", "", "", false
	}

	name = c.Param("name")
	if name == "" {
		jsonError(c, http.StatusBadRequest, "name is required")
		return "", "", "", false
	}

	return singular, scope, name, true
}
