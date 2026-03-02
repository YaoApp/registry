package auth

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func tempAuthPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "test.auth")
}

func TestAddAndVerifyUser(t *testing.T) {
	af := NewAuthFile(tempAuthPath(t))
	if err := af.AddUser("admin", "secret123"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	if !af.Verify("admin", "secret123") {
		t.Error("Verify returned false for correct password")
	}
	if af.Verify("admin", "wrong") {
		t.Error("Verify returned true for wrong password")
	}
}

func TestAddUser_Duplicate(t *testing.T) {
	af := NewAuthFile(tempAuthPath(t))
	if err := af.AddUser("admin", "pass1"); err != nil {
		t.Fatalf("first AddUser: %v", err)
	}
	err := af.AddUser("admin", "pass2")
	if err != ErrUserExists {
		t.Errorf("duplicate AddUser err = %v, want ErrUserExists", err)
	}
}

func TestRemoveUser(t *testing.T) {
	af := NewAuthFile(tempAuthPath(t))
	af.AddUser("admin", "pass")
	af.AddUser("ci-bot", "pass2")

	if err := af.RemoveUser("ci-bot"); err != nil {
		t.Fatalf("RemoveUser: %v", err)
	}
	if af.Verify("ci-bot", "pass2") {
		t.Error("removed user still verifies")
	}
	if !af.Verify("admin", "pass") {
		t.Error("remaining user no longer verifies")
	}
}

func TestRemoveUser_NotFound(t *testing.T) {
	af := NewAuthFile(tempAuthPath(t))
	err := af.RemoveUser("ghost")
	if err != ErrUserNotFound {
		t.Errorf("RemoveUser err = %v, want ErrUserNotFound", err)
	}
}

func TestUpdatePassword(t *testing.T) {
	af := NewAuthFile(tempAuthPath(t))
	af.AddUser("admin", "oldpass")
	if err := af.UpdatePassword("admin", "newpass"); err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}
	if af.Verify("admin", "oldpass") {
		t.Error("old password still works")
	}
	if !af.Verify("admin", "newpass") {
		t.Error("new password does not work")
	}
}

func TestUpdatePassword_NotFound(t *testing.T) {
	af := NewAuthFile(tempAuthPath(t))
	err := af.UpdatePassword("ghost", "pass")
	if err != ErrUserNotFound {
		t.Errorf("UpdatePassword err = %v, want ErrUserNotFound", err)
	}
}

func TestListUsers(t *testing.T) {
	af := NewAuthFile(tempAuthPath(t))
	af.AddUser("beta", "p")
	af.AddUser("alpha", "p")
	af.AddUser("gamma", "p")

	users := af.ListUsers()
	sort.Strings(users)
	want := []string{"alpha", "beta", "gamma"}
	if len(users) != len(want) {
		t.Fatalf("ListUsers len = %d, want %d", len(users), len(want))
	}
	for i, u := range users {
		if u != want[i] {
			t.Errorf("user[%d] = %q, want %q", i, u, want[i])
		}
	}
}

func TestHasUsers(t *testing.T) {
	af := NewAuthFile(tempAuthPath(t))
	if af.HasUsers() {
		t.Error("HasUsers = true for empty file")
	}
	af.AddUser("admin", "p")
	if !af.HasUsers() {
		t.Error("HasUsers = false after AddUser")
	}
}

func TestLoadSavePersistence(t *testing.T) {
	path := tempAuthPath(t)

	af1 := NewAuthFile(path)
	af1.AddUser("admin", "secret")
	af1.AddUser("bot", "token")

	af2 := NewAuthFile(path)
	if err := af2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !af2.Verify("admin", "secret") {
		t.Error("admin not found after reload")
	}
	if !af2.Verify("bot", "token") {
		t.Error("bot not found after reload")
	}
	if af2.Verify("admin", "wrong") {
		t.Error("wrong password accepted after reload")
	}
}

func TestLoad_NonExistentFile(t *testing.T) {
	af := NewAuthFile(filepath.Join(t.TempDir(), "nonexistent"))
	if err := af.Load(); err != nil {
		t.Errorf("Load of nonexistent file should not error, got %v", err)
	}
	if af.HasUsers() {
		t.Error("HasUsers = true for nonexistent file")
	}
}

func TestLoad_MalformedLines(t *testing.T) {
	path := tempAuthPath(t)
	content := "good:$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012\n" +
		"badline_no_colon\n" +
		":empty_user\n" +
		"empty_hash:\n" +
		"# comment line\n" +
		"\n"
	os.WriteFile(path, []byte(content), 0644)

	af := NewAuthFile(path)
	if err := af.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	users := af.ListUsers()
	if len(users) != 1 || users[0] != "good" {
		t.Errorf("ListUsers = %v, want [good]", users)
	}
}

func TestVerify_NonExistentUser(t *testing.T) {
	af := NewAuthFile(tempAuthPath(t))
	if af.Verify("nobody", "pass") {
		t.Error("Verify returned true for nonexistent user")
	}
}

func TestPath(t *testing.T) {
	p := "/some/path/.auth"
	af := NewAuthFile(p)
	if af.Path() != p {
		t.Errorf("Path() = %q, want %q", af.Path(), p)
	}
}
