package models

import (
	"database/sql"
	"fmt"
	"time"
)

// Package represents a row in the packages table.
type Package struct {
	ID          int64
	Type        string
	Scope       string
	Name        string
	Description string
	Keywords    string // JSON array
	Icon        string
	License     string
	Author      string // JSON object
	Maintainers string // JSON array
	Homepage    string
	Repository  string // JSON object
	Bugs        string // JSON object
	Readme      string
	DistTags    string // JSON object
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// UpsertPackage inserts a new package or updates the existing one's metadata.
// On conflict (type, scope, name), it updates description, keywords, and other
// metadata fields, and explicitly sets updated_at.
func UpsertPackage(tx *sql.Tx, pkg *Package) (int64, error) {
	_, err := tx.Exec(`
		INSERT INTO packages (type, scope, name, description, keywords, icon, license,
			author, maintainers, homepage, repository, bugs, readme, dist_tags)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(type, scope, name) DO UPDATE SET
			description = excluded.description,
			keywords    = excluded.keywords,
			icon        = excluded.icon,
			license     = excluded.license,
			author      = excluded.author,
			maintainers = excluded.maintainers,
			homepage    = excluded.homepage,
			repository  = excluded.repository,
			bugs        = excluded.bugs,
			readme      = excluded.readme,
			updated_at  = CURRENT_TIMESTAMP`,
		pkg.Type, pkg.Scope, pkg.Name, pkg.Description, pkg.Keywords,
		pkg.Icon, pkg.License, pkg.Author, pkg.Maintainers,
		pkg.Homepage, pkg.Repository, pkg.Bugs, pkg.Readme, pkg.DistTags,
	)
	if err != nil {
		return 0, fmt.Errorf("upsert package: %w", err)
	}

	// Always query the real ID — LastInsertId is unreliable with ON CONFLICT
	// because SQLite may return a phantom autoincrement value.
	var id int64
	err = tx.QueryRow(
		`SELECT id FROM packages WHERE type=? AND scope=? AND name=?`,
		pkg.Type, pkg.Scope, pkg.Name,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("fetch package id: %w", err)
	}
	return id, nil
}

// GetPackage retrieves a single package by type, scope, and name.
func GetPackage(db *sql.DB, pkgType, scope, name string) (*Package, error) {
	p := &Package{}
	err := db.QueryRow(`
		SELECT id, type, scope, name, description, keywords, icon, license,
			author, maintainers, homepage, repository, bugs, readme, dist_tags,
			created_at, updated_at
		FROM packages WHERE type=? AND scope=? AND name=?`,
		pkgType, scope, name,
	).Scan(&p.ID, &p.Type, &p.Scope, &p.Name, &p.Description, &p.Keywords,
		&p.Icon, &p.License, &p.Author, &p.Maintainers, &p.Homepage,
		&p.Repository, &p.Bugs, &p.Readme, &p.DistTags, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// GetPackageByID retrieves a package by its primary key.
func GetPackageByID(db *sql.DB, id int64) (*Package, error) {
	p := &Package{}
	err := db.QueryRow(`
		SELECT id, type, scope, name, description, keywords, icon, license,
			author, maintainers, homepage, repository, bugs, readme, dist_tags,
			created_at, updated_at
		FROM packages WHERE id=?`, id,
	).Scan(&p.ID, &p.Type, &p.Scope, &p.Name, &p.Description, &p.Keywords,
		&p.Icon, &p.License, &p.Author, &p.Maintainers, &p.Homepage,
		&p.Repository, &p.Bugs, &p.Readme, &p.DistTags, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// PackageListResult holds paginated results from ListPackages.
type PackageListResult struct {
	Packages []*Package
	Total    int
}

// ListPackages returns packages of the given type, optionally filtered by scope
// and a search query, with pagination.
func ListPackages(db *sql.DB, pkgType, scope, q string, page, pageSize int) (*PackageListResult, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	where := "WHERE type=?"
	args := []any{pkgType}

	if scope != "" {
		where += " AND scope=?"
		args = append(args, scope)
	}
	if q != "" {
		where += " AND (name LIKE ? OR description LIKE ? OR keywords LIKE ?)"
		pattern := "%" + q + "%"
		args = append(args, pattern, pattern, pattern)
	}

	var total int
	err := db.QueryRow("SELECT COUNT(*) FROM packages "+where, args...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("count packages: %w", err)
	}

	offset := (page - 1) * pageSize
	rows, err := db.Query(
		"SELECT id, type, scope, name, description, keywords, icon, license, "+
			"author, maintainers, homepage, repository, bugs, readme, dist_tags, "+
			"created_at, updated_at FROM packages "+where+
			" ORDER BY updated_at DESC LIMIT ? OFFSET ?",
		append(args, pageSize, offset)...,
	)
	if err != nil {
		return nil, fmt.Errorf("list packages: %w", err)
	}
	defer rows.Close()

	var pkgs []*Package
	for rows.Next() {
		p := &Package{}
		if err := rows.Scan(&p.ID, &p.Type, &p.Scope, &p.Name, &p.Description,
			&p.Keywords, &p.Icon, &p.License, &p.Author, &p.Maintainers,
			&p.Homepage, &p.Repository, &p.Bugs, &p.Readme, &p.DistTags,
			&p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan package: %w", err)
		}
		pkgs = append(pkgs, p)
	}
	return &PackageListResult{Packages: pkgs, Total: total}, nil
}

// SearchPackages searches across all package types by name, scope, description,
// or keywords.
func SearchPackages(db *sql.DB, q, pkgType string, page, pageSize int) (*PackageListResult, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	where := "WHERE (name LIKE ? OR scope LIKE ? OR description LIKE ? OR keywords LIKE ?)"
	pattern := "%" + q + "%"
	args := []any{pattern, pattern, pattern, pattern}

	if pkgType != "" {
		where += " AND type=?"
		args = append(args, pkgType)
	}

	var total int
	err := db.QueryRow("SELECT COUNT(*) FROM packages "+where, args...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("count search: %w", err)
	}

	offset := (page - 1) * pageSize
	rows, err := db.Query(
		"SELECT id, type, scope, name, description, keywords, icon, license, "+
			"author, maintainers, homepage, repository, bugs, readme, dist_tags, "+
			"created_at, updated_at FROM packages "+where+
			" ORDER BY updated_at DESC LIMIT ? OFFSET ?",
		append(args, pageSize, offset)...,
	)
	if err != nil {
		return nil, fmt.Errorf("search packages: %w", err)
	}
	defer rows.Close()

	var pkgs []*Package
	for rows.Next() {
		p := &Package{}
		if err := rows.Scan(&p.ID, &p.Type, &p.Scope, &p.Name, &p.Description,
			&p.Keywords, &p.Icon, &p.License, &p.Author, &p.Maintainers,
			&p.Homepage, &p.Repository, &p.Bugs, &p.Readme, &p.DistTags,
			&p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan search: %w", err)
		}
		pkgs = append(pkgs, p)
	}
	return &PackageListResult{Packages: pkgs, Total: total}, nil
}

// UpdateDistTags updates the dist_tags JSON field for a package.
func UpdateDistTags(tx *sql.Tx, pkgID int64, distTags string) error {
	_, err := tx.Exec(
		`UPDATE packages SET dist_tags=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		distTags, pkgID,
	)
	return err
}
