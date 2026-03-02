package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yaoapp/registry/auth"
	"github.com/yaoapp/registry/config"
	"github.com/yaoapp/registry/models"
	"github.com/yaoapp/registry/pack"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestServer(t *testing.T) (*Server, *gin.Engine) {
	t.Helper()
	db, err := models.TestDB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	tmpDir := t.TempDir()
	af := auth.NewAuthFile(filepath.Join(tmpDir, ".auth"))
	af.AddUser("admin", "secret")

	cfg := &config.Config{
		DataPath: filepath.Join(tmpDir, "storage"),
		MaxSize:  10, // 10 MB for tests
	}

	s := &Server{DB: db, Config: cfg, AuthFile: af}
	r := gin.New()
	s.SetupRoutes(r)
	return s, r
}

func authHeader() string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:secret"))
}

func makeTestZip(t *testing.T, pkgType, scope, name, version string) []byte {
	t.Helper()
	zipData, err := pack.CreateTestZip(&pack.PkgYao{
		Type: pkgType, Scope: scope, Name: name, Version: version,
		Description: "Test package",
		Keywords:    []string{"test"},
		License:     "MIT",
		Author:      &pack.PersonInfo{Name: "Tester"},
	}, map[string][]byte{
		"package/README.md": []byte("# " + name),
	})
	if err != nil {
		t.Fatal(err)
	}
	return zipData
}

func makeTestZipWithDeps(t *testing.T, pkgType, scope, name, version string, deps []pack.PkgDependency) []byte {
	t.Helper()
	zipData, err := pack.CreateTestZip(&pack.PkgYao{
		Type: pkgType, Scope: scope, Name: name, Version: version,
		Description:  "Test package with deps",
		Dependencies: deps,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return zipData
}

func pushPackage(t *testing.T, r *gin.Engine, typePlural, scope, name, version string, zipData []byte) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	url := fmt.Sprintf("/v1/%s/%s/%s/%s", typePlural, scope, name, version)
	req, _ := http.NewRequest("PUT", url, bytes.NewReader(zipData))
	req.Header.Set("Authorization", authHeader())
	req.Header.Set("Content-Type", "application/zip")
	r.ServeHTTP(w, req)
	return w
}

func getJSON(t *testing.T, r *gin.Engine, url string) (int, map[string]interface{}) {
	t.Helper()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", url, nil)
	r.ServeHTTP(w, req)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	return w.Code, body
}

// --- WellKnown ---

func TestWellKnown(t *testing.T) {
	_, r := setupTestServer(t)
	code, body := getJSON(t, r, "/.well-known/yao-registry")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	reg, ok := body["registry"].(map[string]interface{})
	if !ok {
		t.Fatal("missing registry field")
	}
	if reg["api"] != "/v1" {
		t.Errorf("api = %v", reg["api"])
	}
}

// --- Info ---

func TestInfo(t *testing.T) {
	_, r := setupTestServer(t)
	code, body := getJSON(t, r, "/v1/")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	if body["name"] != "yao-registry" {
		t.Errorf("name = %v", body["name"])
	}
}

// --- Push ---

func TestPush_Success(t *testing.T) {
	_, r := setupTestServer(t)
	zipData := makeTestZip(t, "assistant", "@yao", "keeper", "1.0.0")
	w := pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0", zipData)

	if w.Code != 201 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["version"] != "1.0.0" {
		t.Errorf("version = %v", body["version"])
	}
	if body["digest"] == nil || body["digest"] == "" {
		t.Error("missing digest")
	}
}

func TestPush_DuplicateVersion(t *testing.T) {
	_, r := setupTestServer(t)
	zipData := makeTestZip(t, "assistant", "@yao", "keeper", "1.0.0")
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0", zipData)
	w := pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0", zipData)

	if w.Code != 409 {
		t.Errorf("status = %d, want 409", w.Code)
	}
}

func TestPush_InvalidType(t *testing.T) {
	_, r := setupTestServer(t)
	zipData := makeTestZip(t, "assistant", "@yao", "keeper", "1.0.0")
	w := pushPackage(t, r, "invalid", "@yao", "keeper", "1.0.0", zipData)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestPush_NoAuth(t *testing.T) {
	_, r := setupTestServer(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/v1/assistants/@yao/keeper/1.0.0", nil)
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestPush_TypeMismatch(t *testing.T) {
	_, r := setupTestServer(t)
	zipData := makeTestZip(t, "mcp", "@yao", "keeper", "1.0.0")
	w := pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0", zipData)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestPush_ScopeNoAt(t *testing.T) {
	_, r := setupTestServer(t)
	zipData := makeTestZip(t, "assistant", "yao", "keeper", "1.0.0")
	w := pushPackage(t, r, "assistants", "yao", "keeper", "1.0.0", zipData)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// --- List ---

func TestList(t *testing.T) {
	_, r := setupTestServer(t)
	pushPackage(t, r, "assistants", "@yao", "a", "1.0.0",
		makeTestZip(t, "assistant", "@yao", "a", "1.0.0"))
	pushPackage(t, r, "assistants", "@yao", "b", "1.0.0",
		makeTestZip(t, "assistant", "@yao", "b", "1.0.0"))

	code, body := getJSON(t, r, "/v1/assistants")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	total := body["total"].(float64)
	if total != 2 {
		t.Errorf("total = %v", total)
	}
}

func TestList_InvalidType(t *testing.T) {
	_, r := setupTestServer(t)
	code, _ := getJSON(t, r, "/v1/invalid")
	if code != 400 {
		t.Errorf("status = %d, want 400", code)
	}
}

// --- Search ---

func TestSearch(t *testing.T) {
	_, r := setupTestServer(t)
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0",
		makeTestZip(t, "assistant", "@yao", "keeper", "1.0.0"))
	pushPackage(t, r, "mcps", "@yao", "data-tools", "1.0.0",
		makeTestZip(t, "mcp", "@yao", "data-tools", "1.0.0"))

	code, body := getJSON(t, r, "/v1/search?q=keeper")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	if body["total"].(float64) != 1 {
		t.Errorf("total = %v", body["total"])
	}
}

func TestSearch_MissingQ(t *testing.T) {
	_, r := setupTestServer(t)
	code, _ := getJSON(t, r, "/v1/search")
	if code != 400 {
		t.Errorf("status = %d, want 400", code)
	}
}

// --- Packument ---

func TestPackument(t *testing.T) {
	_, r := setupTestServer(t)
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0",
		makeTestZip(t, "assistant", "@yao", "keeper", "1.0.0"))
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.1.0",
		makeTestZip(t, "assistant", "@yao", "keeper", "1.1.0"))

	code, body := getJSON(t, r, "/v1/assistants/@yao/keeper")
	if code != 200 {
		t.Fatalf("status = %d, body = %v", code, body)
	}
	if body["name"] != "keeper" {
		t.Errorf("name = %v", body["name"])
	}
	versions := body["versions"].(map[string]interface{})
	if len(versions) != 2 {
		t.Errorf("versions count = %d, want 2", len(versions))
	}
	distTags := body["dist_tags"].(map[string]interface{})
	if distTags["latest"] != "1.1.0" {
		t.Errorf("latest = %v, want 1.1.0", distTags["latest"])
	}
}

func TestPackument_Abbreviated(t *testing.T) {
	_, r := setupTestServer(t)
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0",
		makeTestZip(t, "assistant", "@yao", "keeper", "1.0.0"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/assistants/@yao/keeper", nil)
	req.Header.Set("Accept", "application/vnd.yao.abbreviated+json")
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)

	// Abbreviated should not have readme, license, etc.
	if _, ok := body["readme"]; ok {
		t.Error("abbreviated should not have readme")
	}
}

func TestPackument_NotFound(t *testing.T) {
	_, r := setupTestServer(t)
	code, _ := getJSON(t, r, "/v1/assistants/@yao/nonexistent")
	if code != 404 {
		t.Errorf("status = %d, want 404", code)
	}
}

// --- VersionDetail ---

func TestVersionDetail(t *testing.T) {
	_, r := setupTestServer(t)
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0",
		makeTestZip(t, "assistant", "@yao", "keeper", "1.0.0"))

	code, body := getJSON(t, r, "/v1/assistants/@yao/keeper/1.0.0")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	if body["version"] != "1.0.0" {
		t.Errorf("version = %v", body["version"])
	}
	if body["digest"] == nil {
		t.Error("missing digest")
	}
}

func TestVersionDetail_NotFound(t *testing.T) {
	_, r := setupTestServer(t)
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0",
		makeTestZip(t, "assistant", "@yao", "keeper", "1.0.0"))

	code, _ := getJSON(t, r, "/v1/assistants/@yao/keeper/9.9.9")
	if code != 404 {
		t.Errorf("status = %d, want 404", code)
	}
}

// --- Pull ---

func TestPull(t *testing.T) {
	_, r := setupTestServer(t)
	zipData := makeTestZip(t, "assistant", "@yao", "keeper", "1.0.0")
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0", zipData)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/assistants/@yao/keeper/1.0.0/pull", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/zip" {
		t.Errorf("Content-Type = %q", w.Header().Get("Content-Type"))
	}
	if w.Header().Get("X-Digest") == "" {
		t.Error("missing X-Digest header")
	}
	if len(w.Body.Bytes()) != len(zipData) {
		t.Errorf("body size = %d, want %d", len(w.Body.Bytes()), len(zipData))
	}
}

func TestPull_ByTag(t *testing.T) {
	_, r := setupTestServer(t)
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0",
		makeTestZip(t, "assistant", "@yao", "keeper", "1.0.0"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/assistants/@yao/keeper/latest/pull", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestPull_NotFound(t *testing.T) {
	_, r := setupTestServer(t)
	code, _ := getJSON(t, r, "/v1/assistants/@yao/keeper/1.0.0/pull")
	if code != 404 {
		t.Errorf("status = %d, want 404", code)
	}
}

// --- Dependencies ---

func TestDependencies(t *testing.T) {
	_, r := setupTestServer(t)
	zipData := makeTestZipWithDeps(t, "assistant", "@yao", "keeper", "1.0.0", []pack.PkgDependency{
		{Type: "mcp", Scope: "@yao", Name: "tools", Version: "^1.0.0"},
	})
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0", zipData)

	code, body := getJSON(t, r, "/v1/assistants/@yao/keeper/1.0.0/dependencies")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	deps := body["dependencies"].([]interface{})
	if len(deps) != 1 {
		t.Errorf("deps len = %d", len(deps))
	}
}

func TestDependencies_Recursive(t *testing.T) {
	_, r := setupTestServer(t)

	// Push keeper-tools first (no deps)
	pushPackage(t, r, "mcps", "@yao", "tools", "1.0.0",
		makeTestZip(t, "mcp", "@yao", "tools", "1.0.0"))

	// Push keeper which depends on tools
	zipData := makeTestZipWithDeps(t, "assistant", "@yao", "keeper", "1.0.0", []pack.PkgDependency{
		{Type: "mcp", Scope: "@yao", Name: "tools", Version: "^1.0.0"},
	})
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0", zipData)

	code, body := getJSON(t, r, "/v1/assistants/@yao/keeper/1.0.0/dependencies?recursive=true")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	deps := body["dependencies"].([]interface{})
	if len(deps) != 1 {
		t.Errorf("tree len = %d, want 1", len(deps))
	}
}

// --- Dependents ---

func TestDependents(t *testing.T) {
	_, r := setupTestServer(t)

	pushPackage(t, r, "mcps", "@yao", "tools", "1.0.0",
		makeTestZip(t, "mcp", "@yao", "tools", "1.0.0"))

	zipData := makeTestZipWithDeps(t, "assistant", "@yao", "keeper", "1.0.0", []pack.PkgDependency{
		{Type: "mcp", Scope: "@yao", Name: "tools", Version: "^1.0.0"},
	})
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0", zipData)

	code, body := getJSON(t, r, "/v1/mcps/@yao/tools/dependents")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	dependents := body["dependents"].([]interface{})
	if len(dependents) != 1 {
		t.Errorf("dependents len = %d", len(dependents))
	}
}

// --- Tags ---

func TestTagSet(t *testing.T) {
	_, r := setupTestServer(t)
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0",
		makeTestZip(t, "assistant", "@yao", "keeper", "1.0.0"))

	w := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"version": "1.0.0"})
	req, _ := http.NewRequest("PUT", "/v1/assistants/@yao/keeper/tags/canary", bytes.NewReader(body))
	req.Header.Set("Authorization", authHeader())
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	// Verify tag resolves
	code, pkgBody := getJSON(t, r, "/v1/assistants/@yao/keeper")
	if code != 200 {
		t.Fatalf("packument status = %d", code)
	}
	distTags := pkgBody["dist_tags"].(map[string]interface{})
	if distTags["canary"] != "1.0.0" {
		t.Errorf("canary = %v", distTags["canary"])
	}
}

func TestTagDelete(t *testing.T) {
	_, r := setupTestServer(t)
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0",
		makeTestZip(t, "assistant", "@yao", "keeper", "1.0.0"))

	// Set canary tag
	body, _ := json.Marshal(map[string]string{"version": "1.0.0"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/v1/assistants/@yao/keeper/tags/canary", bytes.NewReader(body))
	req.Header.Set("Authorization", authHeader())
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// Delete canary tag
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("DELETE", "/v1/assistants/@yao/keeper/tags/canary", nil)
	req2.Header.Set("Authorization", authHeader())
	r.ServeHTTP(w2, req2)

	if w2.Code != 200 {
		t.Fatalf("status = %d", w2.Code)
	}
}

func TestTagDelete_Latest_Forbidden(t *testing.T) {
	_, r := setupTestServer(t)
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0",
		makeTestZip(t, "assistant", "@yao", "keeper", "1.0.0"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/v1/assistants/@yao/keeper/tags/latest", nil)
	req.Header.Set("Authorization", authHeader())
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// --- Delete ---

func TestDelete_Version(t *testing.T) {
	_, r := setupTestServer(t)
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0",
		makeTestZip(t, "assistant", "@yao", "keeper", "1.0.0"))
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.1.0",
		makeTestZip(t, "assistant", "@yao", "keeper", "1.1.0"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/v1/assistants/@yao/keeper/1.0.0", nil)
	req.Header.Set("Authorization", authHeader())
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	// Verify it's gone
	code, _ := getJSON(t, r, "/v1/assistants/@yao/keeper/1.0.0")
	if code != 404 {
		t.Errorf("after delete: status = %d, want 404", code)
	}

	// Verify 1.1.0 still exists
	code2, _ := getJSON(t, r, "/v1/assistants/@yao/keeper/1.1.0")
	if code2 != 200 {
		t.Errorf("1.1.0 status = %d, want 200", code2)
	}
}

func TestDelete_NoAuth(t *testing.T) {
	_, r := setupTestServer(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/v1/assistants/@yao/keeper/1.0.0", nil)
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

// --- Helpers ---

func TestValidateType(t *testing.T) {
	tests := []struct {
		input    string
		wantErr  bool
		singular string
	}{
		{"releases", false, "release"},
		{"robots", false, "robot"},
		{"assistants", false, "assistant"},
		{"mcps", false, "mcp"},
		{"invalid", true, ""},
		{"", true, ""},
	}
	for _, tt := range tests {
		s, err := validateType(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateType(%q) err = %v, wantErr = %v", tt.input, err, tt.wantErr)
		}
		if s != tt.singular {
			t.Errorf("validateType(%q) = %q, want %q", tt.input, s, tt.singular)
		}
	}
}

func TestParseScope(t *testing.T) {
	if _, err := parseScope("@yao"); err != nil {
		t.Errorf("parseScope(@yao) err = %v", err)
	}
	if _, err := parseScope("yao"); err == nil {
		t.Error("parseScope(yao) should fail")
	}
}

func TestSingularToPlural(t *testing.T) {
	if singularToPlural("assistant") != "assistants" {
		t.Errorf("got %q", singularToPlural("assistant"))
	}
	if singularToPlural("release") != "releases" {
		t.Errorf("got %q", singularToPlural("release"))
	}
}

// --- Push with too-large body ---

func TestPush_TooLarge(t *testing.T) {
	_, r := setupTestServer(t)
	// MaxSize is 10 MB; create a body > 10MB
	oversized := make([]byte, 10*1024*1024+1)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/v1/assistants/@yao/keeper/1.0.0",
		bytes.NewReader(oversized))
	req.Header.Set("Authorization", authHeader())
	req.Header.Set("Content-Type", "application/zip")
	r.ServeHTTP(w, req)

	if w.Code != 413 {
		t.Errorf("status = %d, want 413", w.Code)
	}
}
