package auth

import (
	"path/filepath"
	"testing"
)

func TestSave_Explicit(t *testing.T) {
	path := tempAuthPath(t)
	af := NewAuthFile(path)
	af.AddUser("admin", "pass")

	if err := af.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	af2 := NewAuthFile(path)
	af2.Load()
	if !af2.Verify("admin", "pass") {
		t.Error("user not persisted after Save")
	}
}

func TestLoad_UnreadableFile(t *testing.T) {
	// Point to a directory path instead of file (will fail to open)
	af := NewAuthFile(t.TempDir())
	err := af.Load()
	if err == nil {
		t.Error("Load of directory should error")
	}
}

func TestSaveUnlocked_CreateDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deep", "nested", ".auth")
	af := NewAuthFile(path)
	if err := af.AddUser("admin", "pass"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}

	af2 := NewAuthFile(path)
	af2.Load()
	if !af2.Verify("admin", "pass") {
		t.Error("user not persisted in nested dir")
	}
}

func TestAddUser_MultipleThenList(t *testing.T) {
	af := NewAuthFile(tempAuthPath(t))
	for _, u := range []string{"alice", "bob", "charlie", "dave", "eve"} {
		if err := af.AddUser(u, "pass-"+u); err != nil {
			t.Fatalf("AddUser(%q): %v", u, err)
		}
	}
	users := af.ListUsers()
	if len(users) != 5 {
		t.Errorf("ListUsers len = %d, want 5", len(users))
	}
	for _, u := range []string{"alice", "bob", "charlie", "dave", "eve"} {
		if !af.Verify(u, "pass-"+u) {
			t.Errorf("Verify(%q) failed", u)
		}
	}
}
