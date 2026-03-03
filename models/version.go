package models

import (
	"database/sql"
	"fmt"
	"time"
)

// Version represents a row in the versions table.
type Version struct {
	ID        int64
	PackageID int64
	Version   string
	OS        string
	Arch      string
	Variant   string
	Digest    string
	Size      int64
	Metadata  string // JSON object
	FilePath  string
	CreatedAt time.Time
}

// InsertVersion inserts a new version row. Returns the new row ID.
func InsertVersion(db *sql.DB, v *Version) (int64, error) {
	res, err := db.Exec(`
		INSERT INTO versions (package_id, version, os, arch, variant, digest, size, metadata, file_path)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		v.PackageID, v.Version, v.OS, v.Arch, v.Variant,
		v.Digest, v.Size, v.Metadata, v.FilePath,
	)
	if err != nil {
		return 0, fmt.Errorf("insert version: %w", err)
	}
	return res.LastInsertId()
}

// GetVersion retrieves a specific version by package ID, version string, and
// optional platform identifiers.
func GetVersion(db *sql.DB, packageID int64, version, os, arch, variant string) (*Version, error) {
	v := &Version{}
	err := db.QueryRow(`
		SELECT id, package_id, version, os, arch, variant, digest, size, metadata, file_path, created_at
		FROM versions WHERE package_id=? AND version=? AND os=? AND arch=? AND variant=?`,
		packageID, version, os, arch, variant,
	).Scan(&v.ID, &v.PackageID, &v.Version, &v.OS, &v.Arch, &v.Variant,
		&v.Digest, &v.Size, &v.Metadata, &v.FilePath, &v.CreatedAt)
	if err != nil {
		return nil, err
	}
	return v, nil
}

// GetVersionByID retrieves a version by its primary key.
func GetVersionByID(db *sql.DB, id int64) (*Version, error) {
	v := &Version{}
	err := db.QueryRow(`
		SELECT id, package_id, version, os, arch, variant, digest, size, metadata, file_path, created_at
		FROM versions WHERE id=?`, id,
	).Scan(&v.ID, &v.PackageID, &v.Version, &v.OS, &v.Arch, &v.Variant,
		&v.Digest, &v.Size, &v.Metadata, &v.FilePath, &v.CreatedAt)
	if err != nil {
		return nil, err
	}
	return v, nil
}

// ListVersions returns all versions for a given package, ordered by creation.
func ListVersions(db *sql.DB, packageID int64) ([]*Version, error) {
	rows, err := db.Query(`
		SELECT id, package_id, version, os, arch, variant, digest, size, metadata, file_path, created_at
		FROM versions WHERE package_id=? ORDER BY created_at DESC`, packageID,
	)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer rows.Close()

	var versions []*Version
	for rows.Next() {
		v := &Version{}
		if err := rows.Scan(&v.ID, &v.PackageID, &v.Version, &v.OS, &v.Arch, &v.Variant,
			&v.Digest, &v.Size, &v.Metadata, &v.FilePath, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		versions = append(versions, v)
	}
	return versions, nil
}

// ListVersionsByPlatform returns versions matching specific platform criteria.
func ListVersionsByPlatform(db *sql.DB, packageID int64, version, os, arch, variant string) ([]*Version, error) {
	where := "WHERE package_id=?"
	args := []any{packageID}
	if version != "" {
		where += " AND version=?"
		args = append(args, version)
	}
	if os != "" {
		where += " AND os=?"
		args = append(args, os)
	}
	if arch != "" {
		where += " AND arch=?"
		args = append(args, arch)
	}
	if variant != "" {
		where += " AND variant=?"
		args = append(args, variant)
	}

	rows, err := db.Query(`
		SELECT id, package_id, version, os, arch, variant, digest, size, metadata, file_path, created_at
		FROM versions `+where+` ORDER BY created_at DESC`, args...,
	)
	if err != nil {
		return nil, fmt.Errorf("list versions by platform: %w", err)
	}
	defer rows.Close()

	var versions []*Version
	for rows.Next() {
		v := &Version{}
		if err := rows.Scan(&v.ID, &v.PackageID, &v.Version, &v.OS, &v.Arch, &v.Variant,
			&v.Digest, &v.Size, &v.Metadata, &v.FilePath, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		versions = append(versions, v)
	}
	return versions, nil
}

// DeleteVersion removes a version row by ID.
func DeleteVersion(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM versions WHERE id=?`, id)
	return err
}

// CountVersions returns the number of versions for a package.
func CountVersions(db *sql.DB, packageID int64) (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM versions WHERE package_id=?`, packageID).Scan(&count)
	return count, err
}

// GetLatestNonPrerelease returns the most recent version string that does not
// contain a hyphen (i.e. no prerelease suffix), useful for dist-tag fallback.
func GetLatestNonPrerelease(db *sql.DB, packageID int64) (string, error) {
	var ver string
	err := db.QueryRow(`
		SELECT version FROM versions
		WHERE package_id=? AND version NOT LIKE '%-%'
		ORDER BY id DESC LIMIT 1`, packageID,
	).Scan(&ver)
	return ver, err
}
