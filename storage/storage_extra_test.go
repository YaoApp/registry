package storage

import (
	"io"
	"testing"
)

func TestStore_CreatesNestedDirs(t *testing.T) {
	base := t.TempDir()
	data := []byte("deep nested")

	relPath, err := Store(base, "assistants", "@community", "deep-pkg", "1.0.0", "", "", "", data)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	rc, size, err := Load(base, relPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer rc.Close()

	if size != int64(len(data)) {
		t.Errorf("size = %d", size)
	}
	got, _ := io.ReadAll(rc)
	if string(got) != "deep nested" {
		t.Errorf("content = %q", got)
	}
}

func TestStore_MultiplePlatforms(t *testing.T) {
	base := t.TempDir()

	platforms := [][3]string{
		{"linux", "amd64", "prod"},
		{"linux", "arm64", "prod"},
		{"darwin", "arm64", ""},
		{"windows", "amd64", ""},
	}

	for _, p := range platforms {
		data := []byte("binary-" + p[0] + "-" + p[1])
		_, err := Store(base, "releases", "@yao", "yao", "1.0.0", p[0], p[1], p[2], data)
		if err != nil {
			t.Fatalf("Store %v: %v", p, err)
		}
	}

	// Verify each can be loaded
	for _, p := range platforms {
		data := []byte("binary-" + p[0] + "-" + p[1])
		relPath, _ := Store(base, "releases", "@yao", "yao", "1.0.0", p[0], p[1], p[2], data)
		rc, _, err := Load(base, relPath)
		if err != nil {
			t.Fatalf("Load %v: %v", p, err)
		}
		rc.Close()
	}
}

func TestComputeDigest_Empty(t *testing.T) {
	d := ComputeDigest([]byte{})
	if d == "" {
		t.Error("empty data should produce a valid digest")
	}
	if len(d) != 7+64 {
		t.Errorf("digest length = %d", len(d))
	}
}

func TestEnsureDir_Idempotent(t *testing.T) {
	dir := t.TempDir() + "/subdir"
	if err := EnsureDir(dir); err != nil {
		t.Fatalf("first EnsureDir: %v", err)
	}
	if err := EnsureDir(dir); err != nil {
		t.Fatalf("second EnsureDir: %v", err)
	}
}
