package models

import (
	"database/sql"
	"fmt"
)

// Dependency represents a row in the dependencies table.
type Dependency struct {
	ID         int64
	VersionID  int64
	DepType    string
	DepScope   string
	DepName    string
	DepVersion string // semver constraint (e.g. "^1.0.0")
	Optional   bool
}

// DependencyTreeNode represents a node in a resolved dependency tree.
type DependencyTreeNode struct {
	Type              string   `json:"type"`
	Scope             string   `json:"scope"`
	Name              string   `json:"name"`
	VersionConstraint string   `json:"version_constraint"`
	Resolved          string   `json:"resolved,omitempty"`
	RequiredBy        []string `json:"required_by"`
	Circular          bool     `json:"circular,omitempty"`
}

// InsertDependencies bulk-inserts dependencies for a given version.
func InsertDependencies(tx *sql.Tx, versionID int64, deps []Dependency) error {
	stmt, err := tx.Prepare(`
		INSERT INTO dependencies (version_id, dep_type, dep_scope, dep_name, dep_version, optional)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare insert dep: %w", err)
	}
	defer stmt.Close()

	for _, d := range deps {
		optional := 0
		if d.Optional {
			optional = 1
		}
		if _, err := stmt.Exec(versionID, d.DepType, d.DepScope, d.DepName, d.DepVersion, optional); err != nil {
			return fmt.Errorf("insert dep %s/%s/%s: %w", d.DepType, d.DepScope, d.DepName, err)
		}
	}
	return nil
}

// GetDependencies returns the direct dependencies for a version.
func GetDependencies(db *sql.DB, versionID int64) ([]Dependency, error) {
	rows, err := db.Query(`
		SELECT id, version_id, dep_type, dep_scope, dep_name, dep_version, optional
		FROM dependencies WHERE version_id=?`, versionID)
	if err != nil {
		return nil, fmt.Errorf("query deps: %w", err)
	}
	defer rows.Close()

	var deps []Dependency
	for rows.Next() {
		var d Dependency
		var opt int
		if err := rows.Scan(&d.ID, &d.VersionID, &d.DepType, &d.DepScope, &d.DepName, &d.DepVersion, &opt); err != nil {
			return nil, fmt.Errorf("scan dep: %w", err)
		}
		d.Optional = opt == 1
		deps = append(deps, d)
	}
	return deps, nil
}

// Dependent represents a package that depends on a given target.
type Dependent struct {
	Type    string
	Scope   string
	Name    string
	Version string // the version of the dependent, not the constraint
}

// GetDependents returns packages that depend on the given target (reverse lookup).
func GetDependents(db *sql.DB, depType, depScope, depName string) ([]Dependent, error) {
	rows, err := db.Query(`
		SELECT p.type, p.scope, p.name, v.version
		FROM dependencies d
		JOIN versions v ON v.id = d.version_id
		JOIN packages p ON p.id = v.package_id
		WHERE d.dep_type=? AND d.dep_scope=? AND d.dep_name=?
		ORDER BY p.scope, p.name, v.version`,
		depType, depScope, depName)
	if err != nil {
		return nil, fmt.Errorf("query dependents: %w", err)
	}
	defer rows.Close()

	var result []Dependent
	for rows.Next() {
		var d Dependent
		if err := rows.Scan(&d.Type, &d.Scope, &d.Name, &d.Version); err != nil {
			return nil, fmt.Errorf("scan dependent: %w", err)
		}
		result = append(result, d)
	}
	return result, nil
}

// depKey creates a unique identifier for cycle detection.
func depKey(typ, scope, name, constraint string) string {
	return typ + ":" + scope + "/" + name + "@" + constraint
}

// ResolveDependencyTree builds a flat dependency tree using BFS with cycle
// detection. It resolves each dependency to the latest matching version stored
// in the registry.
func ResolveDependencyTree(db *sql.DB, versionID int64) ([]DependencyTreeNode, error) {
	type queueItem struct {
		versionID  int64
		requiredBy string
	}

	// Get the root version to form the "required_by" label.
	rootVer, err := GetVersionByID(db, versionID)
	if err != nil {
		return nil, fmt.Errorf("get root version: %w", err)
	}
	rootPkg, err := GetPackageByID(db, rootVer.PackageID)
	if err != nil {
		return nil, fmt.Errorf("get root package: %w", err)
	}
	rootLabel := rootPkg.Type + ":" + rootPkg.Scope + "/" + rootPkg.Name + "@" + rootVer.Version

	visited := map[string]*DependencyTreeNode{}
	queue := []queueItem{{versionID: versionID, requiredBy: rootLabel}}

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		deps, err := GetDependencies(db, item.versionID)
		if err != nil {
			return nil, err
		}

		for _, d := range deps {
			key := depKey(d.DepType, d.DepScope, d.DepName, d.DepVersion)

			if node, exists := visited[key]; exists {
				node.RequiredBy = append(node.RequiredBy, item.requiredBy)
				continue
			}

			node := &DependencyTreeNode{
				Type:              d.DepType,
				Scope:             d.DepScope,
				Name:              d.DepName,
				VersionConstraint: d.DepVersion,
				RequiredBy:        []string{item.requiredBy},
			}
			visited[key] = node

			// Try to resolve: find the package and pick its latest version.
			pkg, err := GetPackage(db, d.DepType, d.DepScope, d.DepName)
			if err != nil {
				// Unresolved dependency — leave Resolved empty.
				continue
			}

			versions, err := ListVersions(db, pkg.ID)
			if err != nil || len(versions) == 0 {
				continue
			}

			// Use the most recently published version as the resolved version.
			resolved := versions[0]
			node.Resolved = resolved.Version

			// Check for circular dependency: if this resolved version is the
			// same as our root or if we've already queued it.
			circularKey := d.DepType + ":" + d.DepScope + "/" + d.DepName + "@" + resolved.Version
			if circularKey == rootLabel {
				node.Circular = true
				continue
			}

			queue = append(queue, queueItem{
				versionID:  resolved.ID,
				requiredBy: d.DepType + ":" + d.DepScope + "/" + d.DepName + "@" + resolved.Version,
			})
		}
	}

	result := make([]DependencyTreeNode, 0, len(visited))
	for _, node := range visited {
		result = append(result, *node)
	}
	return result, nil
}

// DeleteDependenciesByVersion removes all dependencies for a given version.
func DeleteDependenciesByVersion(tx *sql.Tx, versionID int64) error {
	_, err := tx.Exec(`DELETE FROM dependencies WHERE version_id=?`, versionID)
	return err
}
