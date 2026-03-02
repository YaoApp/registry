package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yaoapp/registry/pack"
)

// Cover versionKey with variant
func TestVersionKey_WithVariant(t *testing.T) {
	_, r := setupTestServer(t)
	zipData, _ := pack.CreateTestZip(&pack.PkgYao{
		Type: "release", Scope: "@yao", Name: "yao", Version: "1.0.0",
		Platform: &pack.PkgPlatform{OS: "linux", Arch: "amd64", Variant: "prod"},
	}, map[string][]byte{"package/bin/yao": []byte("binary")})
	pushPackage(t, r, "releases", "@yao", "yao", "1.0.0", zipData)

	code, body := getJSON(t, r, "/v1/releases/@yao/yao")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	versions := body["versions"].(map[string]interface{})
	if _, ok := versions["1.0.0-linux-amd64-prod"]; !ok {
		t.Errorf("expected key 1.0.0-linux-amd64-prod, got keys: %v", mapKeys(versions))
	}
}

// Cover versionKey without variant (OS + Arch only)
func TestVersionKey_NoVariant(t *testing.T) {
	_, r := setupTestServer(t)
	zipData, _ := pack.CreateTestZip(&pack.PkgYao{
		Type: "release", Scope: "@yao", Name: "yao", Version: "1.0.0",
		Platform: &pack.PkgPlatform{OS: "darwin", Arch: "arm64"},
	}, map[string][]byte{"package/bin/yao": []byte("binary")})
	pushPackage(t, r, "releases", "@yao", "yao", "1.0.0", zipData)

	code, body := getJSON(t, r, "/v1/releases/@yao/yao")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	versions := body["versions"].(map[string]interface{})
	if _, ok := versions["1.0.0-darwin-arm64"]; !ok {
		t.Errorf("expected key 1.0.0-darwin-arm64, got keys: %v", mapKeys(versions))
	}
}

// Cover singularToPlural unknown type
func TestSingularToPlural_Unknown(t *testing.T) {
	got := singularToPlural("unknown")
	if got != "unknown" {
		t.Errorf("singularToPlural(unknown) = %q, want unknown", got)
	}
}

// Cover Dependencies with recursive tree from handler
func TestDependencies_RecursiveHandler(t *testing.T) {
	_, r := setupTestServer(t)
	pushPackage(t, r, "mcps", "@yao", "tools", "1.0.0",
		makeTestZip(t, "mcp", "@yao", "tools", "1.0.0"))
	pushPackage(t, r, "assistants", "@yao", "translator", "2.0.0",
		makeTestZip(t, "assistant", "@yao", "translator", "2.0.0"))

	zipData := makeTestZipWithDeps(t, "assistant", "@yao", "keeper", "1.0.0", []pack.PkgDependency{
		{Type: "mcp", Scope: "@yao", Name: "tools", Version: "^1.0.0"},
		{Type: "assistant", Scope: "@yao", Name: "translator", Version: "~2.0.0"},
	})
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0", zipData)

	code, body := getJSON(t, r, "/v1/assistants/@yao/keeper/1.0.0/dependencies?recursive=true")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	deps := body["dependencies"].([]interface{})
	if len(deps) != 2 {
		t.Errorf("deps = %d, want 2", len(deps))
	}
}

// Cover Dependents with multiple dependents
func TestDependents_Multiple(t *testing.T) {
	_, r := setupTestServer(t)
	pushPackage(t, r, "mcps", "@yao", "tools", "1.0.0",
		makeTestZip(t, "mcp", "@yao", "tools", "1.0.0"))

	for _, name := range []string{"a", "b", "c"} {
		zipData := makeTestZipWithDeps(t, "assistant", "@yao", name, "1.0.0", []pack.PkgDependency{
			{Type: "mcp", Scope: "@yao", Name: "tools", Version: "^1.0.0"},
		})
		pushPackage(t, r, "assistants", "@yao", name, "1.0.0", zipData)
	}

	code, body := getJSON(t, r, "/v1/mcps/@yao/tools/dependents")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	deps := body["dependents"].([]interface{})
	if len(deps) != 3 {
		t.Errorf("dependents = %d, want 3", len(deps))
	}
}

// Cover Packument with release platform entries (abbreviated mode)
func TestPackument_ReleaseAbbreviated(t *testing.T) {
	_, r := setupTestServer(t)
	zipData, _ := pack.CreateTestZip(&pack.PkgYao{
		Type: "release", Scope: "@yao", Name: "yao", Version: "1.0.0",
		Platform: &pack.PkgPlatform{OS: "linux", Arch: "amd64"},
	}, nil)
	pushPackage(t, r, "releases", "@yao", "yao", "1.0.0", zipData)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/releases/@yao/yao", nil)
	req.Header.Set("Accept", "application/vnd.yao.abbreviated+json")
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	versions := body["versions"].(map[string]interface{})
	entry := versions["1.0.0-linux-amd64"].(map[string]interface{})
	if entry["os"] != "linux" {
		t.Errorf("os = %v", entry["os"])
	}
}

// Cover Push with invalid zip (not a valid zip archive)
func TestPush_InvalidZip(t *testing.T) {
	_, r := setupTestServer(t)
	w := pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0", []byte("not a zip"))
	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// Cover Push with version mismatch
func TestPush_VersionMismatch(t *testing.T) {
	_, r := setupTestServer(t)
	zipData := makeTestZip(t, "assistant", "@yao", "keeper", "1.0.0")
	w := pushPackage(t, r, "assistants", "@yao", "keeper", "2.0.0", zipData)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// Cover Delete with tag cleanup for non-latest tag
func TestDelete_RemovesCustomTag(t *testing.T) {
	_, r := setupTestServer(t)
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0",
		makeTestZip(t, "assistant", "@yao", "keeper", "1.0.0"))
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.1.0",
		makeTestZip(t, "assistant", "@yao", "keeper", "1.1.0"))

	// Set canary tag to 1.0.0
	body, _ := json.Marshal(map[string]string{"version": "1.0.0"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/v1/assistants/@yao/keeper/tags/canary",
		bytes.NewReader(body))
	req.Header.Set("Authorization", authHeader())
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// Delete 1.0.0 — canary tag should be removed
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("DELETE", "/v1/assistants/@yao/keeper/1.0.0", nil)
	req2.Header.Set("Authorization", authHeader())
	r.ServeHTTP(w2, req2)
	if w2.Code != 200 {
		t.Fatalf("delete status = %d", w2.Code)
	}

	// Verify canary tag is gone
	code, pkgBody := getJSON(t, r, "/v1/assistants/@yao/keeper")
	if code != 200 {
		t.Fatalf("packument status = %d", code)
	}
	distTags := pkgBody["dist_tags"].(map[string]interface{})
	if _, exists := distTags["canary"]; exists {
		t.Error("canary tag should have been removed")
	}
}

// Cover List with query parameter
func TestList_WithQuery(t *testing.T) {
	_, r := setupTestServer(t)
	pushPackage(t, r, "assistants", "@yao", "data-keeper", "1.0.0",
		makeTestZip(t, "assistant", "@yao", "data-keeper", "1.0.0"))
	pushPackage(t, r, "assistants", "@yao", "translator", "1.0.0",
		makeTestZip(t, "assistant", "@yao", "translator", "1.0.0"))

	code, body := getJSON(t, r, "/v1/assistants?q=data")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	if body["total"].(float64) != 1 {
		t.Errorf("total = %v, want 1", body["total"])
	}
}

// Cover Pull for release with platform params
func TestPull_ReleasePlatform(t *testing.T) {
	_, r := setupTestServer(t)
	zipData, _ := pack.CreateTestZip(&pack.PkgYao{
		Type: "release", Scope: "@yao", Name: "yao", Version: "1.0.0",
		Platform: &pack.PkgPlatform{OS: "linux", Arch: "amd64"},
	}, map[string][]byte{"package/bin/yao": []byte("binary")})
	pushPackage(t, r, "releases", "@yao", "yao", "1.0.0", zipData)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/releases/@yao/yao/1.0.0/pull?os=linux&arch=amd64", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if w.Header().Get("X-Digest") == "" {
		t.Error("missing X-Digest")
	}
}

// Cover Packument with engines in abbreviated mode
func TestPackument_AbbreviatedWithEngines(t *testing.T) {
	_, r := setupTestServer(t)
	zipData, _ := pack.CreateTestZip(&pack.PkgYao{
		Type: "assistant", Scope: "@yao", Name: "keeper", Version: "1.0.0",
		Engines:  map[string]string{"yao": ">=2.0.0"},
		Metadata: map[string]interface{}{"engines": map[string]string{"yao": ">=2.0.0"}},
	}, nil)
	pushPackage(t, r, "assistants", "@yao", "keeper", "1.0.0", zipData)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/assistants/@yao/keeper", nil)
	req.Header.Set("Accept", "application/vnd.yao.abbreviated+json")
	r.ServeHTTP(w, req)

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	versions := body["versions"].(map[string]interface{})
	v := versions["1.0.0"].(map[string]interface{})
	if v["engines"] == nil {
		t.Error("abbreviated should include engines")
	}
}

func mapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
