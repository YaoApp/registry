package storage

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreAndLoad(t *testing.T) {
	base := t.TempDir()
	data := []byte("fake zip content")

	relPath, err := Store(base, "assistants", "@yao", "keeper", "1.0.0", "", "", "", data)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if relPath != filepath.Join("assistants", "@yao", "keeper", "1.0.0.yao.zip") {
		t.Errorf("relPath = %q", relPath)
	}

	rc, size, err := Load(base, relPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer rc.Close()

	if size != int64(len(data)) {
		t.Errorf("size = %d, want %d", size, len(data))
	}

	got, _ := io.ReadAll(rc)
	if string(got) != "fake zip content" {
		t.Errorf("content = %q", got)
	}
}

func TestStore_Release(t *testing.T) {
	base := t.TempDir()
	data := []byte("binary")

	relPath, err := Store(base, "releases", "@yao", "yao", "1.0.0", "linux", "amd64", "prod", data)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	expected := filepath.Join("releases", "@yao", "yao", "1.0.0-linux-amd64-prod.yao.zip")
	if relPath != expected {
		t.Errorf("relPath = %q, want %q", relPath, expected)
	}

	if _, err := os.Stat(filepath.Join(base, relPath)); err != nil {
		t.Errorf("file not found: %v", err)
	}
}

func TestStore_ReleaseNoVariant(t *testing.T) {
	base := t.TempDir()
	relPath, err := Store(base, "releases", "@yao", "yao", "1.0.0", "darwin", "arm64", "", []byte("bin"))
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	expected := filepath.Join("releases", "@yao", "yao", "1.0.0-darwin-arm64.yao.zip")
	if relPath != expected {
		t.Errorf("relPath = %q, want %q", relPath, expected)
	}
}

func TestDelete(t *testing.T) {
	base := t.TempDir()
	data := []byte("to be deleted")

	relPath, _ := Store(base, "mcps", "@yao", "tool", "1.0.0", "", "", "", data)
	if err := Delete(base, relPath); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, _, err := Load(base, relPath)
	if err == nil {
		t.Error("Load should fail after delete")
	}
}

func TestDelete_NonExistent(t *testing.T) {
	base := t.TempDir()
	if err := Delete(base, "nonexistent.zip"); err != nil {
		t.Errorf("Delete nonexistent should not error: %v", err)
	}
}

func TestComputeDigest(t *testing.T) {
	data := []byte("hello world")
	digest := ComputeDigest(data)
	if !strings.HasPrefix(digest, "sha256:") {
		t.Errorf("digest should start with sha256:, got %q", digest)
	}
	if len(digest) != 7+64 {
		t.Errorf("digest length = %d, want %d", len(digest), 7+64)
	}

	// Same data should produce the same digest
	digest2 := ComputeDigest(data)
	if digest != digest2 {
		t.Errorf("digest not deterministic")
	}

	// Different data should produce different digest
	digest3 := ComputeDigest([]byte("different"))
	if digest == digest3 {
		t.Errorf("different data produced same digest")
	}
}

func TestEnsureDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "deep", "nested", "dir")
	if err := EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("not a directory")
	}
}

func TestLoad_NotFound(t *testing.T) {
	base := t.TempDir()
	_, _, err := Load(base, "does-not-exist.zip")
	if err == nil {
		t.Error("Load should fail for missing file")
	}
}
