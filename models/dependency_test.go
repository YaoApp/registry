package models

import (
	"database/sql"
	"testing"
)

func setupVersionForDep(t *testing.T, db *sql.DB, pkgType, scope, name, ver string) (pkgID, verID int64) {
	t.Helper()
	pkgID, err := UpsertPackage(db, &Package{
		Type: pkgType, Scope: scope, Name: name,
		Keywords: "[]", DistTags: "{}",
	})
	if err != nil {
		t.Fatalf("setup package: %v", err)
	}
	verID, err = InsertVersion(db, &Version{
		PackageID: pkgID, Version: ver,
		Digest: "sha256:test", Size: 100,
		Metadata: "{}", FilePath: name + ".zip",
	})
	if err != nil {
		t.Fatalf("setup version: %v", err)
	}
	return
}

func TestInsertAndGetDependencies(t *testing.T) {
	db, _ := TestDB()
	defer db.Close()
	_, verID := setupVersionForDep(t, db, "assistant", "@yao", "keeper", "1.0.0")

	deps := []Dependency{
		{DepType: "mcp", DepScope: "@yao", DepName: "keeper-tools", DepVersion: "^1.0.0"},
		{DepType: "assistant", DepScope: "@yao", DepName: "translator", DepVersion: "~2.0.0", Optional: true},
	}

	if err := InsertDependencies(db, verID, deps); err != nil {
		t.Fatalf("InsertDependencies: %v", err)
	}

	got, err := GetDependencies(db, verID)
	if err != nil {
		t.Fatalf("GetDependencies: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}

	found := map[string]Dependency{}
	for _, d := range got {
		found[d.DepName] = d
	}

	kt := found["keeper-tools"]
	if kt.DepType != "mcp" || kt.DepScope != "@yao" || kt.DepVersion != "^1.0.0" {
		t.Errorf("keeper-tools = %+v", kt)
	}
	if kt.Optional {
		t.Error("keeper-tools should not be optional")
	}

	tr := found["translator"]
	if !tr.Optional {
		t.Error("translator should be optional")
	}
}

func TestGetDependencies_Empty(t *testing.T) {
	db, _ := TestDB()
	defer db.Close()
	_, verID := setupVersionForDep(t, db, "mcp", "@yao", "tool", "1.0.0")

	deps, err := GetDependencies(db, verID)
	if err != nil {
		t.Fatalf("GetDependencies: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("len = %d, want 0", len(deps))
	}
}

func TestGetDependents(t *testing.T) {
	db, _ := TestDB()
	defer db.Close()

	_, keeperVerID := setupVersionForDep(t, db, "assistant", "@yao", "keeper", "1.0.0")
	setupVersionForDep(t, db, "mcp", "@yao", "keeper-tools", "1.2.0")

	InsertDependencies(db, keeperVerID, []Dependency{
		{DepType: "mcp", DepScope: "@yao", DepName: "keeper-tools", DepVersion: "^1.0.0"},
	})

	dependents, err := GetDependents(db, "mcp", "@yao", "keeper-tools")
	if err != nil {
		t.Fatalf("GetDependents: %v", err)
	}
	if len(dependents) != 1 {
		t.Fatalf("len = %d, want 1", len(dependents))
	}
	if dependents[0].Name != "keeper" || dependents[0].Version != "1.0.0" {
		t.Errorf("dependent = %+v", dependents[0])
	}
}

func TestGetDependents_Empty(t *testing.T) {
	db, _ := TestDB()
	defer db.Close()

	deps, err := GetDependents(db, "mcp", "@yao", "nonexistent")
	if err != nil {
		t.Fatalf("GetDependents: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("len = %d, want 0", len(deps))
	}
}

func TestResolveDependencyTree(t *testing.T) {
	db, _ := TestDB()
	defer db.Close()

	_, keeperVerID := setupVersionForDep(t, db, "assistant", "@yao", "keeper", "1.0.0")
	_, ktVerID := setupVersionForDep(t, db, "mcp", "@yao", "keeper-tools", "1.2.0")
	setupVersionForDep(t, db, "assistant", "@yao", "translator", "2.0.0")

	InsertDependencies(db, keeperVerID, []Dependency{
		{DepType: "mcp", DepScope: "@yao", DepName: "keeper-tools", DepVersion: "^1.0.0"},
	})
	InsertDependencies(db, ktVerID, []Dependency{
		{DepType: "assistant", DepScope: "@yao", DepName: "translator", DepVersion: "~2.0.0"},
	})

	tree, err := ResolveDependencyTree(db, keeperVerID)
	if err != nil {
		t.Fatalf("ResolveDependencyTree: %v", err)
	}
	if len(tree) != 2 {
		t.Fatalf("tree len = %d, want 2", len(tree))
	}

	nodeMap := map[string]DependencyTreeNode{}
	for _, n := range tree {
		nodeMap[n.Name] = n
	}

	kt := nodeMap["keeper-tools"]
	if kt.Resolved != "1.2.0" {
		t.Errorf("keeper-tools resolved = %q, want 1.2.0", kt.Resolved)
	}
	if len(kt.RequiredBy) != 1 || kt.RequiredBy[0] != "assistant:@yao/keeper@1.0.0" {
		t.Errorf("keeper-tools required_by = %v", kt.RequiredBy)
	}

	tr := nodeMap["translator"]
	if tr.Resolved != "2.0.0" {
		t.Errorf("translator resolved = %q, want 2.0.0", tr.Resolved)
	}
}

func TestResolveDependencyTree_CircularDetection(t *testing.T) {
	db, _ := TestDB()
	defer db.Close()

	_, aVerID := setupVersionForDep(t, db, "mcp", "@yao", "tool-a", "1.0.0")
	_, bVerID := setupVersionForDep(t, db, "mcp", "@yao", "tool-b", "1.0.0")

	InsertDependencies(db, aVerID, []Dependency{
		{DepType: "mcp", DepScope: "@yao", DepName: "tool-b", DepVersion: "^1.0.0"},
	})
	InsertDependencies(db, bVerID, []Dependency{
		{DepType: "mcp", DepScope: "@yao", DepName: "tool-a", DepVersion: "^1.0.0"},
	})

	tree, err := ResolveDependencyTree(db, aVerID)
	if err != nil {
		t.Fatalf("ResolveDependencyTree: %v", err)
	}

	hasCircular := false
	for _, n := range tree {
		if n.Circular {
			hasCircular = true
			if n.Name != "tool-a" {
				t.Errorf("circular node = %q, want tool-a", n.Name)
			}
		}
	}
	if !hasCircular {
		t.Error("expected circular dependency to be detected")
	}
}

func TestDeleteDependenciesByVersion(t *testing.T) {
	db, _ := TestDB()
	defer db.Close()
	_, verID := setupVersionForDep(t, db, "assistant", "@yao", "keeper", "1.0.0")

	InsertDependencies(db, verID, []Dependency{
		{DepType: "mcp", DepScope: "@yao", DepName: "tool", DepVersion: "^1.0.0"},
	})

	deps, _ := GetDependencies(db, verID)
	if len(deps) != 1 {
		t.Fatalf("before delete: len = %d", len(deps))
	}

	if err := DeleteDependenciesByVersion(db, verID); err != nil {
		t.Fatalf("DeleteDependenciesByVersion: %v", err)
	}

	deps, _ = GetDependencies(db, verID)
	if len(deps) != 0 {
		t.Errorf("after delete: len = %d, want 0", len(deps))
	}
}
