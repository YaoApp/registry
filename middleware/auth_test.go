package middleware

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yaoapp/registry/auth"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupAuth(t *testing.T) *auth.AuthFile {
	t.Helper()
	af := auth.NewAuthFile(filepath.Join(t.TempDir(), ".auth"))
	if err := af.AddUser("admin", "secret"); err != nil {
		t.Fatalf("add user: %v", err)
	}
	return af
}

func setupRouter(af *auth.AuthFile) *gin.Engine {
	r := gin.New()
	r.PUT("/push", BasicAuth(af), func(c *gin.Context) {
		user, _ := c.Get("username")
		c.JSON(http.StatusOK, gin.H{"user": user})
	})
	return r
}

func basicAuth(user, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}

func TestBasicAuth_ValidCredentials(t *testing.T) {
	af := setupAuth(t)
	r := setupRouter(af)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/push", nil)
	req.Header.Set("Authorization", basicAuth("admin", "secret"))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["user"] != "admin" {
		t.Errorf("user = %q, want admin", body["user"])
	}
}

func TestBasicAuth_InvalidPassword(t *testing.T) {
	af := setupAuth(t)
	r := setupRouter(af)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/push", nil)
	req.Header.Set("Authorization", basicAuth("admin", "wrong"))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}

	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["error"] != "invalid username or password" {
		t.Errorf("error = %q", body["error"])
	}
}

func TestBasicAuth_MissingHeader(t *testing.T) {
	af := setupAuth(t)
	r := setupRouter(af)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/push", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
	if w.Header().Get("WWW-Authenticate") == "" {
		t.Error("missing WWW-Authenticate header")
	}
}

func TestBasicAuth_WrongScheme(t *testing.T) {
	af := setupAuth(t)
	r := setupRouter(af)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/push", nil)
	req.Header.Set("Authorization", "Bearer token123")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestBasicAuth_MalformedBase64(t *testing.T) {
	af := setupAuth(t)
	r := setupRouter(af)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/push", nil)
	req.Header.Set("Authorization", "Basic !!!invalid!!!")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestBasicAuth_NoColon(t *testing.T) {
	af := setupAuth(t)
	r := setupRouter(af)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/push", nil)
	encoded := base64.StdEncoding.EncodeToString([]byte("nocolon"))
	req.Header.Set("Authorization", "Basic "+encoded)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestBasicAuth_UnknownUser(t *testing.T) {
	af := setupAuth(t)
	r := setupRouter(af)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/push", nil)
	req.Header.Set("Authorization", basicAuth("unknown", "secret"))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}
