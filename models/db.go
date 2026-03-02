// Package models provides database initialization and CRUD operations for the
// registry's three core tables: packages, versions, and dependencies.
package models

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// InitDB opens (or creates) the SQLite database at dbPath, enables WAL mode,
// and creates all tables and indexes if they don't already exist.
func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable WAL: %w", err)
	}

	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if err := createTables(db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func createTables(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS packages (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			type        TEXT NOT NULL,
			scope       TEXT NOT NULL,
			name        TEXT NOT NULL,
			description TEXT DEFAULT '',
			keywords    TEXT DEFAULT '[]',
			icon        TEXT DEFAULT '',
			license     TEXT DEFAULT '',
			author      TEXT DEFAULT '{}',
			maintainers TEXT DEFAULT '[]',
			homepage    TEXT DEFAULT '',
			repository  TEXT DEFAULT '{}',
			bugs        TEXT DEFAULT '{}',
			readme      TEXT DEFAULT '',
			dist_tags   TEXT DEFAULT '{}',
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(type, scope, name)
		)`,
		`CREATE TABLE IF NOT EXISTS versions (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			package_id  INTEGER NOT NULL REFERENCES packages(id),
			version     TEXT NOT NULL,
			os          TEXT DEFAULT '',
			arch        TEXT DEFAULT '',
			variant     TEXT DEFAULT '',
			digest      TEXT NOT NULL,
			size        INTEGER NOT NULL,
			metadata    TEXT DEFAULT '{}',
			file_path   TEXT NOT NULL,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(package_id, version, os, arch, variant)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_versions_package ON versions(package_id)`,
		`CREATE INDEX IF NOT EXISTS idx_versions_platform ON versions(os, arch, variant)`,
		`CREATE TABLE IF NOT EXISTS dependencies (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			version_id      INTEGER NOT NULL REFERENCES versions(id) ON DELETE CASCADE,
			dep_type        TEXT NOT NULL,
			dep_scope       TEXT NOT NULL,
			dep_name        TEXT NOT NULL,
			dep_version     TEXT DEFAULT '',
			optional        INTEGER DEFAULT 0,
			created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_deps_version ON dependencies(version_id)`,
		`CREATE INDEX IF NOT EXISTS idx_deps_target ON dependencies(dep_type, dep_scope, dep_name)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("exec DDL: %w\nSQL: %s", err, s)
		}
	}
	return nil
}

// TestDB creates an in-memory SQLite database for testing.
func TestDB() (*sql.DB, error) {
	return InitDB(":memory:")
}
