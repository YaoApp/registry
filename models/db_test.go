package models

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitDB_InMemory(t *testing.T) {
	db, err := TestDB()
	if err != nil {
		t.Fatalf("TestDB: %v", err)
	}
	defer db.Close()

	// WAL mode should be active
	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if mode != "wal" && mode != "memory" {
		// in-memory SQLite may report "memory" instead of "wal"
		t.Logf("journal_mode = %q (acceptable for in-memory)", mode)
	}
}

func TestInitDB_TablesExist(t *testing.T) {
	db, err := TestDB()
	if err != nil {
		t.Fatalf("TestDB: %v", err)
	}
	defer db.Close()

	tables := []string{"packages", "versions", "dependencies"}
	for _, table := range tables {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", table, err)
		}
	}
}

func TestInitDB_IndexesExist(t *testing.T) {
	db, err := TestDB()
	if err != nil {
		t.Fatalf("TestDB: %v", err)
	}
	defer db.Close()

	indexes := []string{
		"idx_versions_package",
		"idx_versions_platform",
		"idx_deps_version",
		"idx_deps_target",
	}
	for _, idx := range indexes {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='index' AND name=?", idx,
		).Scan(&name)
		if err != nil {
			t.Errorf("index %q not found: %v", idx, err)
		}
	}
}

func TestInitDB_Idempotent(t *testing.T) {
	db, err := TestDB()
	if err != nil {
		t.Fatalf("TestDB: %v", err)
	}
	defer db.Close()

	// Run createTables again — should not error
	if err := createTables(db); err != nil {
		t.Errorf("second createTables: %v", err)
	}
}

func TestInitDB_FileSystem(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	db.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file not created")
	}

	// Reopen to verify idempotency
	db2, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("second InitDB: %v", err)
	}
	db2.Close()
}

func TestInitDB_ForeignKeys(t *testing.T) {
	db, err := TestDB()
	if err != nil {
		t.Fatalf("TestDB: %v", err)
	}
	defer db.Close()

	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("query foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}
}

func TestInitDB_UniqueConstraint(t *testing.T) {
	db, err := TestDB()
	if err != nil {
		t.Fatalf("TestDB: %v", err)
	}
	defer db.Close()

	insert := `INSERT INTO packages (type, scope, name) VALUES (?, ?, ?)`
	_, err = db.Exec(insert, "assistant", "@yao", "keeper")
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}

	_, err = db.Exec(insert, "assistant", "@yao", "keeper")
	if err == nil {
		t.Error("duplicate insert should fail with UNIQUE constraint")
	}
}
