// Package e2e provides end-to-end black-box tests for the registry server.
// Each test starts a full Gin server with in-memory database and uses only
// net/http client calls.
package e2e

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yaoapp/registry/auth"
	"github.com/yaoapp/registry/config"
	"github.com/yaoapp/registry/handlers"
	"github.com/yaoapp/registry/models"
	"github.com/yaoapp/registry/pack"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type testEnv struct {
	server *httptest.Server
	url    string
}

func setup(t *testing.T) *testEnv {
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
		MaxSize:  1,
	}

	r := gin.New()
	s := &handlers.Server{DB: db, Config: cfg, AuthFile: af}
	s.SetupRoutes(r)

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)

	return &testEnv{server: ts, url: ts.URL}
}

func e2eAuthHeader() string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:secret"))
}

func buildZip(t *testing.T, pkgType, scope, name, version string) []byte {
	t.Helper()
	data, err := pack.CreateTestZip(&pack.PkgYao{
		Type: pkgType, Scope: scope, Name: name, Version: version,
		Description: "E2E test package " + name + "@" + version,
		Keywords:    []string{"e2e"},
		License:     "MIT",
		Author:      &pack.PersonInfo{Name: "Test"},
	}, map[string][]byte{
		"package/README.md": []byte("# " + name),
	})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func buildZipWithDeps(t *testing.T, pkgType, scope, name, version string, deps []pack.PkgDependency) []byte {
	t.Helper()
	data, err := pack.CreateTestZip(&pack.PkgYao{
		Type: pkgType, Scope: scope, Name: name, Version: version,
		Description:  "E2E test package with deps",
		Dependencies: deps,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func buildReleaseZip(t *testing.T, scope, name, version, goos, arch, variant string) []byte {
	t.Helper()
	data, err := pack.CreateTestZip(&pack.PkgYao{
		Type: "release", Scope: scope, Name: name, Version: version,
		Description: "Release binary",
		Platform:    &pack.PkgPlatform{OS: goos, Arch: arch, Variant: variant},
	}, map[string][]byte{
		"package/bin/yao": []byte("fake-binary-" + goos + "-" + arch),
	})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func doPush(t *testing.T, env *testEnv, typePlural, scope, name, version string, body []byte) (int, map[string]interface{}) {
	t.Helper()
	url := fmt.Sprintf("%s/v1/%s/%s/%s/%s", env.url, typePlural, scope, name, version)
	req, _ := http.NewRequest("PUT", url, bytes.NewReader(body))
	req.Header.Set("Authorization", e2eAuthHeader())
	req.Header.Set("Content-Type", "application/zip")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	return resp.StatusCode, result
}

func mustPush(t *testing.T, env *testEnv, typePlural, scope, name, version string, body []byte) {
	t.Helper()
	code, result := doPush(t, env, typePlural, scope, name, version, body)
	if code != 201 {
		t.Fatalf("push %s/%s/%s@%s: status=%d body=%v", typePlural, scope, name, version, code, result)
	}
}

func doGet(t *testing.T, env *testEnv, path string) (int, map[string]interface{}) {
	t.Helper()
	resp, err := http.Get(env.url + path)
	if err != nil {
		t.Fatalf("get %s: %v", path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var body map[string]interface{}
	json.Unmarshal(data, &body)
	return resp.StatusCode, body
}

func doGetRaw(t *testing.T, env *testEnv, path string) *http.Response {
	t.Helper()
	resp, err := http.Get(env.url + path)
	if err != nil {
		t.Fatalf("get %s: %v", path, err)
	}
	return resp
}

// --- Test 1: Push and Pull ---

func TestE2E_PushAndPull(t *testing.T) {
	env := setup(t)
	zipData := buildZip(t, "assistant", "@yao", "keeper", "1.0.0")

	code, pushBody := doPush(t, env, "assistants", "@yao", "keeper", "1.0.0", zipData)
	if code != 201 {
		t.Fatalf("push status = %d, body = %v", code, pushBody)
	}
	if pushBody["digest"] == nil || pushBody["digest"] == "" {
		t.Error("push response missing digest")
	}

	// Pull
	pullResp := doGetRaw(t, env, "/v1/assistants/@yao/keeper/1.0.0/pull")
	defer pullResp.Body.Close()
	if pullResp.StatusCode != 200 {
		t.Fatalf("pull status = %d", pullResp.StatusCode)
	}
	if pullResp.Header.Get("Content-Type") != "application/zip" {
		t.Errorf("Content-Type = %q", pullResp.Header.Get("Content-Type"))
	}
	if pullResp.Header.Get("X-Digest") == "" {
		t.Error("missing X-Digest header")
	}
	pulledData, _ := io.ReadAll(pullResp.Body)
	if len(pulledData) != len(zipData) {
		t.Errorf("pulled size = %d, want %d", len(pulledData), len(zipData))
	}
}

// --- Test 2: Version Lifecycle ---

func TestE2E_VersionLifecycle(t *testing.T) {
	env := setup(t)

	mustPush(t, env, "assistants", "@yao", "keeper", "1.0.0",
		buildZip(t, "assistant", "@yao", "keeper", "1.0.0"))
	mustPush(t, env, "assistants", "@yao", "keeper", "1.1.0",
		buildZip(t, "assistant", "@yao", "keeper", "1.1.0"))

	// List versions via packument
	code, body := doGet(t, env, "/v1/assistants/@yao/keeper")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	versions := body["versions"].(map[string]interface{})
	if len(versions) != 2 {
		t.Errorf("versions = %d, want 2", len(versions))
	}
	distTags := body["dist_tags"].(map[string]interface{})
	if distTags["latest"] != "1.1.0" {
		t.Errorf("latest = %v", distTags["latest"])
	}

	// Delete 1.0.0
	delReq, _ := http.NewRequest("DELETE", env.url+"/v1/assistants/@yao/keeper/1.0.0", nil)
	delReq.Header.Set("Authorization", e2eAuthHeader())
	delResp, _ := http.DefaultClient.Do(delReq)
	if delResp.StatusCode != 200 {
		t.Fatalf("delete status = %d", delResp.StatusCode)
	}
	delResp.Body.Close()

	// 1.0.0 should be 404
	code2, _ := doGet(t, env, "/v1/assistants/@yao/keeper/1.0.0")
	if code2 != 404 {
		t.Errorf("after delete: status = %d, want 404", code2)
	}

	// 1.1.0 should still exist
	code3, _ := doGet(t, env, "/v1/assistants/@yao/keeper/1.1.0")
	if code3 != 200 {
		t.Errorf("1.1.0 status = %d", code3)
	}
}

// --- Test 3: Dist-Tags ---

func TestE2E_DistTags(t *testing.T) {
	env := setup(t)

	mustPush(t, env, "assistants", "@yao", "keeper", "1.0.0",
		buildZip(t, "assistant", "@yao", "keeper", "1.0.0"))

	// Set canary tag
	tagBody, _ := json.Marshal(map[string]string{"version": "1.0.0"})
	req, _ := http.NewRequest("PUT", env.url+"/v1/assistants/@yao/keeper/tags/canary",
		bytes.NewReader(tagBody))
	req.Header.Set("Authorization", e2eAuthHeader())
	req.Header.Set("Content-Type", "application/json")
	tagResp, _ := http.DefaultClient.Do(req)
	if tagResp.StatusCode != 200 {
		t.Fatalf("set tag status = %d", tagResp.StatusCode)
	}
	tagResp.Body.Close()

	// Pull by canary tag
	pullResp := doGetRaw(t, env, "/v1/assistants/@yao/keeper/canary/pull")
	if pullResp.StatusCode != 200 {
		t.Fatalf("pull by tag status = %d", pullResp.StatusCode)
	}
	pullResp.Body.Close()

	// Delete canary tag
	delReq, _ := http.NewRequest("DELETE", env.url+"/v1/assistants/@yao/keeper/tags/canary", nil)
	delReq.Header.Set("Authorization", e2eAuthHeader())
	delResp, _ := http.DefaultClient.Do(delReq)
	if delResp.StatusCode != 200 {
		t.Fatalf("delete tag status = %d", delResp.StatusCode)
	}
	delResp.Body.Close()

	// Latest cannot be deleted
	delLatest, _ := http.NewRequest("DELETE", env.url+"/v1/assistants/@yao/keeper/tags/latest", nil)
	delLatest.Header.Set("Authorization", e2eAuthHeader())
	latestResp, _ := http.DefaultClient.Do(delLatest)
	if latestResp.StatusCode != 400 {
		t.Errorf("delete latest tag status = %d, want 400", latestResp.StatusCode)
	}
	latestResp.Body.Close()
}

// --- Test 4: Search ---

func TestE2E_Search(t *testing.T) {
	env := setup(t)

	mustPush(t, env, "assistants", "@yao", "keeper", "1.0.0",
		buildZip(t, "assistant", "@yao", "keeper", "1.0.0"))
	mustPush(t, env, "mcps", "@yao", "data-tools", "1.0.0",
		buildZip(t, "mcp", "@yao", "data-tools", "1.0.0"))
	mustPush(t, env, "assistants", "@yao", "translator", "1.0.0",
		buildZip(t, "assistant", "@yao", "translator", "1.0.0"))

	// Search by name
	code, body := doGet(t, env, "/v1/search?q=keeper")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	if body["total"].(float64) != 1 {
		t.Errorf("total = %v, want 1", body["total"])
	}

	// Search across types (all have "e2e" keyword)
	_, body2 := doGet(t, env, "/v1/search?q=e2e")
	if body2["total"].(float64) != 3 {
		t.Errorf("total = %v, want 3", body2["total"])
	}

	// Search with pagination
	_, body3 := doGet(t, env, "/v1/search?q=e2e&pagesize=2&page=1")
	pkgs := body3["packages"].([]interface{})
	if len(pkgs) != 2 {
		t.Errorf("page 1 len = %d, want 2", len(pkgs))
	}
}

// --- Test 5: Dependencies ---

func TestE2E_Dependencies(t *testing.T) {
	env := setup(t)

	mustPush(t, env, "mcps", "@yao", "tools", "1.0.0",
		buildZip(t, "mcp", "@yao", "tools", "1.0.0"))

	mustPush(t, env, "assistants", "@yao", "keeper", "1.0.0",
		buildZipWithDeps(t, "assistant", "@yao", "keeper", "1.0.0", []pack.PkgDependency{
			{Type: "mcp", Scope: "@yao", Name: "tools", Version: "^1.0.0"},
		}))

	// Direct dependencies
	code, body := doGet(t, env, "/v1/assistants/@yao/keeper/1.0.0/dependencies")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	deps := body["dependencies"].([]interface{})
	if len(deps) != 1 {
		t.Errorf("deps = %d, want 1", len(deps))
	}

	// Recursive tree
	code2, body2 := doGet(t, env, "/v1/assistants/@yao/keeper/1.0.0/dependencies?recursive=true")
	if code2 != 200 {
		t.Fatalf("recursive status = %d", code2)
	}
	tree := body2["dependencies"].([]interface{})
	if len(tree) != 1 {
		t.Errorf("tree = %d, want 1", len(tree))
	}
	node := tree[0].(map[string]interface{})
	if node["name"] != "tools" {
		t.Errorf("node name = %v", node["name"])
	}
	if node["resolved"] != "1.0.0" {
		t.Errorf("resolved = %v", node["resolved"])
	}
}

// --- Test 6: Auth Required ---

func TestE2E_AuthRequired(t *testing.T) {
	env := setup(t)
	zipData := buildZip(t, "assistant", "@yao", "keeper", "1.0.0")

	// Push without auth
	url := env.url + "/v1/assistants/@yao/keeper/1.0.0"
	req, _ := http.NewRequest("PUT", url, bytes.NewReader(zipData))
	req.Header.Set("Content-Type", "application/zip")
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != 401 {
		t.Errorf("no auth: status = %d, want 401", resp.StatusCode)
	}
	resp.Body.Close()

	// Push with wrong creds
	req2, _ := http.NewRequest("PUT", url, bytes.NewReader(zipData))
	req2.Header.Set("Content-Type", "application/zip")
	req2.Header.Set("Authorization", "Basic "+
		base64.StdEncoding.EncodeToString([]byte("admin:wrong")))
	resp2, _ := http.DefaultClient.Do(req2)
	if resp2.StatusCode != 401 {
		t.Errorf("wrong creds: status = %d, want 401", resp2.StatusCode)
	}
	resp2.Body.Close()
}

// --- Test 7: Packument ---

func TestE2E_Packument(t *testing.T) {
	env := setup(t)

	mustPush(t, env, "assistants", "@yao", "keeper", "1.0.0",
		buildZip(t, "assistant", "@yao", "keeper", "1.0.0"))
	mustPush(t, env, "assistants", "@yao", "keeper", "1.1.0",
		buildZip(t, "assistant", "@yao", "keeper", "1.1.0"))
	mustPush(t, env, "assistants", "@yao", "keeper", "2.0.0",
		buildZip(t, "assistant", "@yao", "keeper", "2.0.0"))

	code, body := doGet(t, env, "/v1/assistants/@yao/keeper")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}

	if body["name"] != "keeper" {
		t.Errorf("name = %v", body["name"])
	}
	if body["type"] != "assistants" {
		t.Errorf("type = %v", body["type"])
	}

	versions := body["versions"].(map[string]interface{})
	if len(versions) != 3 {
		t.Errorf("versions = %d, want 3", len(versions))
	}

	distTags := body["dist_tags"].(map[string]interface{})
	if distTags["latest"] != "2.0.0" {
		t.Errorf("latest = %v, want 2.0.0", distTags["latest"])
	}

	if body["license"] != "MIT" {
		t.Errorf("license = %v", body["license"])
	}
	if body["readme"] != "# keeper" {
		t.Errorf("readme = %v", body["readme"])
	}
}

// --- Test 8: Well-Known ---

func TestE2E_WellKnown(t *testing.T) {
	env := setup(t)

	code, body := doGet(t, env, "/.well-known/yao-registry")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}

	reg := body["registry"].(map[string]interface{})
	if reg["api"] != "/v1" {
		t.Errorf("api = %v", reg["api"])
	}

	types := body["types"].([]interface{})
	if len(types) != 4 {
		t.Errorf("types = %d, want 4", len(types))
	}
}

// --- Test 9: Release Platform ---

func TestE2E_ReleasePlatform(t *testing.T) {
	env := setup(t)

	mustPush(t, env, "releases", "@yao", "yao", "1.0.0",
		buildReleaseZip(t, "@yao", "yao", "1.0.0", "linux", "amd64", ""))
	mustPush(t, env, "releases", "@yao", "yao", "1.0.0",
		buildReleaseZip(t, "@yao", "yao", "1.0.0", "darwin", "arm64", ""))

	// Get version detail — should list all artifacts
	code, body := doGet(t, env, "/v1/releases/@yao/yao/1.0.0")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	artifacts := body["artifacts"].([]interface{})
	if len(artifacts) != 2 {
		t.Errorf("artifacts = %d, want 2", len(artifacts))
	}

	// Filter by OS
	code2, body2 := doGet(t, env, "/v1/releases/@yao/yao/1.0.0?os=linux")
	if code2 != 200 {
		t.Fatalf("filter status = %d", code2)
	}
	artifacts2 := body2["artifacts"].([]interface{})
	if len(artifacts2) != 1 {
		t.Errorf("linux artifacts = %d, want 1", len(artifacts2))
	}

	// Pull specific platform
	pullResp := doGetRaw(t, env, "/v1/releases/@yao/yao/1.0.0/pull?os=darwin&arch=arm64")
	if pullResp.StatusCode != 200 {
		t.Fatalf("pull status = %d", pullResp.StatusCode)
	}
	pullResp.Body.Close()
}

// --- Test 10: Max Size ---

func TestE2E_MaxSize(t *testing.T) {
	env := setup(t)

	// MaxSize is 1 MB; send a body larger than 1MB.
	// Use a real oversized body to trigger the MaxBytesReader limit.
	oversized := make([]byte, 1*1024*1024+1)
	url := env.url + "/v1/assistants/@yao/keeper/1.0.0"
	req, _ := http.NewRequest("PUT", url, bytes.NewReader(oversized))
	req.Header.Set("Authorization", e2eAuthHeader())
	req.Header.Set("Content-Type", "application/zip")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 413 {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("status = %d, want 413, body = %s", resp.StatusCode, body)
	}
}
