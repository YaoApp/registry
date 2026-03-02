package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yaoapp/registry/pack"
)

// --- Additional VersionDetail tests ---

func TestVersionDetail_Release_AllArtifacts(t *testing.T) {
	_, r := setupTestServer(t)

	// Push two platform releases
	pushRelease(t, r, "linux", "amd64", "")
	pushRelease(t, r, "darwin", "arm64", "")

	code, body := getJSON(t, r, "/v1/releases/@yao/yao/1.0.0")
	if code != 200 {
		t.Fatalf("status = %d, body = %v", code, body)
	}
	artifacts := body["artifacts"].([]interface{})
	if len(artifacts) != 2 {
		t.Errorf("artifacts = %d, want 2", len(artifacts))
	}
}

func TestVersionDetail_Release_FilterByOS(t *testing.T) {
	_, r := setupTestServer(t)
	pushRelease(t, r, "linux", "amd64", "")
	pushRelease(t, r, "darwin", "arm64", "")

	code, body := getJSON(t, r, "/v1/releases/@yao/yao/1.0.0?os=linux")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	artifacts := body["artifacts"].([]interface{})
	if len(artifacts) != 1 {
		t.Errorf("artifacts = %d, want 1", len(artifacts))
	}
}

func TestVersionDetail_Release_NoMatch(t *testing.T) {
	_, r := setupTestServer(t)
	pushRelease(t, r, "linux", "amd64", "")

	code, _ := getJSON(t, r, "/v1/releases/@yao/yao/1.0.0?os=windows")
	if code != 404 {
		t.Errorf("status = %d, want 404", code)
	}
}

func TestVersionDetail_Release_VersionNotFound(t *testing.T) {
	_, r := setupTestServer(t)
	pushRelease(t, r, "linux", "amd64", "")

	code, _ := getJSON(t, r, "/v1/releases/@yao/yao/9.9.9")
	if code != 404 {
		t.Errorf("status = %d, want 404", code)
	}
}

func TestVersionDetail_PackageNotFound(t *testing.T) {
	_, r := setupTestServer(t)
	code, _ := getJSON(t, r, "/v1/assistants/@yao/nonexistent/1.0.0")
	if code != 404 {
		t.Errorf("status = %d, want 404", code)
	}
}

func pushRelease(t *testing.T, r *gin.Engine, goos, arch, variant string) {
	t.Helper()
	zipData, _ := pack.CreateTestZip(&pack.PkgYao{
		Type: "release", Scope: "@yao", Name: "yao", Version: "1.0.0",
		Platform: &pack.PkgPlatform{OS: goos, Arch: arch, Variant: variant},
	}, map[string][]byte{
		"package/bin/yao": []byte("binary-" + goos),
	})
	w := pushPackage(t, r, "releases", "@yao", "yao", "1.0.0", zipData)
	if w.Code != 201 {
		t.Fatalf("push release %s/%s: %d %s", goos, arch, w.Code, w.Body.String())
	}
}

// --- Additional Delete tests ---

func TestDelete_NotFound(t *testing.T) {
	_, r := setupTestServer(t)
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0",
		makeTestZip(t, "assistant", "@yao", "keeper", "1.0.0"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/v1/assistants/@yao/keeper/9.9.9", nil)
	req.Header.Set("Authorization", authHeader())
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestDelete_PackageNotFound(t *testing.T) {
	_, r := setupTestServer(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/v1/assistants/@yao/nonexistent/1.0.0", nil)
	req.Header.Set("Authorization", authHeader())
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestDelete_UpdatesLatestTag(t *testing.T) {
	_, r := setupTestServer(t)
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0",
		makeTestZip(t, "assistant", "@yao", "keeper", "1.0.0"))
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.1.0",
		makeTestZip(t, "assistant", "@yao", "keeper", "1.1.0"))

	// Delete the latest version (1.1.0)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/v1/assistants/@yao/keeper/1.1.0", nil)
	req.Header.Set("Authorization", authHeader())
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}

	// Latest should fallback to 1.0.0
	code, body := getJSON(t, r, "/v1/assistants/@yao/keeper")
	if code != 200 {
		t.Fatalf("packument status = %d", code)
	}
	distTags := body["dist_tags"].(map[string]interface{})
	if distTags["latest"] != "1.0.0" {
		t.Errorf("latest = %v, want 1.0.0", distTags["latest"])
	}
}

// --- Additional Tag tests ---

func TestTagSet_PackageNotFound(t *testing.T) {
	_, r := setupTestServer(t)
	body, _ := json.Marshal(map[string]string{"version": "1.0.0"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/v1/assistants/@yao/nonexistent/tags/canary",
		bytes.NewReader(body))
	req.Header.Set("Authorization", authHeader())
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestTagSet_VersionNotFound(t *testing.T) {
	_, r := setupTestServer(t)
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0",
		makeTestZip(t, "assistant", "@yao", "keeper", "1.0.0"))

	body, _ := json.Marshal(map[string]string{"version": "9.9.9"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/v1/assistants/@yao/keeper/tags/canary",
		bytes.NewReader(body))
	req.Header.Set("Authorization", authHeader())
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestTagSet_MissingBody(t *testing.T) {
	_, r := setupTestServer(t)
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0",
		makeTestZip(t, "assistant", "@yao", "keeper", "1.0.0"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/v1/assistants/@yao/keeper/tags/canary", nil)
	req.Header.Set("Authorization", authHeader())
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestTagDelete_NotFound(t *testing.T) {
	_, r := setupTestServer(t)
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0",
		makeTestZip(t, "assistant", "@yao", "keeper", "1.0.0"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/v1/assistants/@yao/keeper/tags/nonexistent", nil)
	req.Header.Set("Authorization", authHeader())
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestTagDelete_PackageNotFound(t *testing.T) {
	_, r := setupTestServer(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/v1/assistants/@yao/nonexistent/tags/canary", nil)
	req.Header.Set("Authorization", authHeader())
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// --- Additional Search tests ---

func TestSearch_WithTypeFilter(t *testing.T) {
	_, r := setupTestServer(t)
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0",
		makeTestZip(t, "assistant", "@yao", "keeper", "1.0.0"))
	pushPackage(t, r, "mcps", "@yao", "data-tools", "1.0.0",
		makeTestZip(t, "mcp", "@yao", "data-tools", "1.0.0"))

	code, body := getJSON(t, r, "/v1/search?q=test&type=assistants")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	if body["total"].(float64) != 1 {
		t.Errorf("total = %v", body["total"])
	}
}

func TestSearch_InvalidType(t *testing.T) {
	_, r := setupTestServer(t)
	code, _ := getJSON(t, r, "/v1/search?q=test&type=invalid")
	if code != 400 {
		t.Errorf("status = %d, want 400", code)
	}
}

// --- Additional Pull tests ---

func TestPull_PackageNotFound(t *testing.T) {
	_, r := setupTestServer(t)
	code, _ := getJSON(t, r, "/v1/assistants/@yao/nonexistent/1.0.0/pull")
	if code != 404 {
		t.Errorf("status = %d, want 404", code)
	}
}

func TestPull_VersionNotFound(t *testing.T) {
	_, r := setupTestServer(t)
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0",
		makeTestZip(t, "assistant", "@yao", "keeper", "1.0.0"))

	code, _ := getJSON(t, r, "/v1/assistants/@yao/keeper/9.9.9/pull")
	if code != 404 {
		t.Errorf("status = %d, want 404", code)
	}
}

// --- Additional Dependencies tests ---

func TestDependencies_PackageNotFound(t *testing.T) {
	_, r := setupTestServer(t)
	code, _ := getJSON(t, r, "/v1/assistants/@yao/nonexistent/1.0.0/dependencies")
	if code != 404 {
		t.Errorf("status = %d, want 404", code)
	}
}

func TestDependencies_VersionNotFound(t *testing.T) {
	_, r := setupTestServer(t)
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0",
		makeTestZip(t, "assistant", "@yao", "keeper", "1.0.0"))

	code, _ := getJSON(t, r, "/v1/assistants/@yao/keeper/9.9.9/dependencies")
	if code != 404 {
		t.Errorf("status = %d, want 404", code)
	}
}

// --- Additional Dependents tests ---

func TestDependents_PackageNoDeps(t *testing.T) {
	_, r := setupTestServer(t)
	pushPackage(t, r, "mcps", "@yao", "tools", "1.0.0",
		makeTestZip(t, "mcp", "@yao", "tools", "1.0.0"))

	code, body := getJSON(t, r, "/v1/mcps/@yao/tools/dependents")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	deps := body["dependents"].([]interface{})
	if len(deps) != 0 {
		t.Errorf("dependents = %d, want 0", len(deps))
	}
}

// --- Push with prerelease version ---

func TestPush_PrereleaseLatestTag(t *testing.T) {
	_, r := setupTestServer(t)

	// First push is a prerelease
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0-beta",
		makeTestZip(t, "assistant", "@yao", "keeper", "1.0.0-beta"))

	code, body := getJSON(t, r, "/v1/assistants/@yao/keeper")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	distTags := body["dist_tags"].(map[string]interface{})
	if distTags["latest"] != "1.0.0-beta" {
		t.Errorf("latest = %v, want 1.0.0-beta (first push)", distTags["latest"])
	}

	// Stable release overwrites latest
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0",
		makeTestZip(t, "assistant", "@yao", "keeper", "1.0.0"))
	_, body2 := getJSON(t, r, "/v1/assistants/@yao/keeper")
	distTags2 := body2["dist_tags"].(map[string]interface{})
	if distTags2["latest"] != "1.0.0" {
		t.Errorf("latest = %v, want 1.0.0", distTags2["latest"])
	}
}

// --- Push with dependencies ---

func TestPush_WithDependencies(t *testing.T) {
	_, r := setupTestServer(t)

	zipData := makeTestZipWithDeps(t, "assistant", "@yao", "keeper", "1.0.0", []pack.PkgDependency{
		{Type: "mcp", Scope: "@yao", Name: "tools", Version: "^1.0.0"},
		{Type: "assistant", Scope: "@yao", Name: "translator", Version: "~2.0.0"},
	})
	w := pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0", zipData)
	if w.Code != 201 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	// Verify dependencies
	code, body := getJSON(t, r, "/v1/assistants/@yao/keeper/1.0.0/dependencies")
	if code != 200 {
		t.Fatalf("deps status = %d", code)
	}
	deps := body["dependencies"].([]interface{})
	if len(deps) != 2 {
		t.Errorf("deps = %d, want 2", len(deps))
	}
}

// --- List with scope filter ---

func TestList_WithScope(t *testing.T) {
	_, r := setupTestServer(t)
	pushPackage(t, r, "assistants", "@yao", "a", "1.0.0",
		makeTestZip(t, "assistant", "@yao", "a", "1.0.0"))
	pushPackage(t, r, "assistants", "@community", "b", "1.0.0",
		makeTestZip(t, "assistant", "@community", "b", "1.0.0"))

	code, body := getJSON(t, r, "/v1/assistants?scope=@yao")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	if body["total"].(float64) != 1 {
		t.Errorf("total = %v, want 1", body["total"])
	}
}

// --- Pagination edge cases ---

func TestBindPagination_Defaults(t *testing.T) {
	_, r := setupTestServer(t)
	pushPackage(t, r, "assistants", "@yao", "a", "1.0.0",
		makeTestZip(t, "assistant", "@yao", "a", "1.0.0"))

	code, body := getJSON(t, r, "/v1/assistants?page=-1&pagesize=0")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	if body["page"].(float64) != 1 {
		t.Errorf("page = %v, want 1", body["page"])
	}
	if body["pagesize"].(float64) != 20 {
		t.Errorf("pagesize = %v, want 20", body["pagesize"])
	}
}

func TestBindPagination_Custom(t *testing.T) {
	_, r := setupTestServer(t)
	for i := 0; i < 25; i++ {
		pushPackage(t, r, "assistants", "@yao", fmt.Sprintf("pkg-%d", i), "1.0.0",
			makeTestZip(t, "assistant", "@yao", fmt.Sprintf("pkg-%d", i), "1.0.0"))
	}

	code, body := getJSON(t, r, "/v1/assistants?page=2&pagesize=10")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	if body["total"].(float64) != 25 {
		t.Errorf("total = %v", body["total"])
	}
	pkgs := body["packages"].([]interface{})
	if len(pkgs) != 10 {
		t.Errorf("page 2 len = %d, want 10", len(pkgs))
	}
}

// --- Push with engines and metadata ---

func TestPush_WithEnginesAndMetadata(t *testing.T) {
	_, r := setupTestServer(t)

	zipData, _ := pack.CreateTestZip(&pack.PkgYao{
		Type: "assistant", Scope: "@yao", Name: "keeper", Version: "1.0.0",
		Engines:  map[string]string{"yao": ">=2.0.0"},
		Metadata: map[string]interface{}{"modes": []string{"chat", "task"}},
	}, nil)
	w := pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0", zipData)
	if w.Code != 201 {
		t.Fatalf("status = %d", w.Code)
	}

	code, body := getJSON(t, r, "/v1/assistants/@yao/keeper/1.0.0")
	if code != 200 {
		t.Fatalf("version detail status = %d", code)
	}
	metadata := body["metadata"].(map[string]interface{})
	engines := metadata["engines"].(map[string]interface{})
	if engines["yao"] != ">=2.0.0" {
		t.Errorf("engines = %v", engines)
	}
}
