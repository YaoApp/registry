package models

import (
	"database/sql"
	"testing"
)

func TestGetLatestNonPrerelease_ViaDB(t *testing.T) {
	db, _ := TestDB()
	defer db.Close()

	pkgID, _ := UpsertPackage(db, &Package{
		Type: "assistant", Scope: "@yao", Name: "keeper",
		Keywords: "[]", DistTags: "{}",
	})
	InsertVersion(db, &Version{
		PackageID: pkgID, Version: "1.0.0",
		Digest: "sha256:a", Size: 100, Metadata: "{}", FilePath: "a.zip",
	})
	InsertVersion(db, &Version{
		PackageID: pkgID, Version: "2.0.0-beta",
		Digest: "sha256:b", Size: 200, Metadata: "{}", FilePath: "b.zip",
	})
	InsertVersion(db, &Version{
		PackageID: pkgID, Version: "1.1.0",
		Digest: "sha256:c", Size: 300, Metadata: "{}", FilePath: "c.zip",
	})

	ver, err := GetLatestNonPrerelease(db, pkgID)
	if err != nil {
		t.Fatalf("GetLatestNonPrerelease: %v", err)
	}
	if ver != "1.1.0" {
		t.Errorf("version = %q, want 1.1.0", ver)
	}
}

func TestGetLatestNonPrerelease_AfterDelete(t *testing.T) {
	db, _ := TestDB()
	defer db.Close()

	pkgID, _ := UpsertPackage(db, &Package{
		Type: "assistant", Scope: "@yao", Name: "keeper",
		Keywords: "[]", DistTags: "{}",
	})
	InsertVersion(db, &Version{
		PackageID: pkgID, Version: "1.0.0",
		Digest: "sha256:a", Size: 100, Metadata: "{}", FilePath: "a.zip",
	})
	id2, _ := InsertVersion(db, &Version{
		PackageID: pkgID, Version: "1.1.0",
		Digest: "sha256:b", Size: 200, Metadata: "{}", FilePath: "b.zip",
	})

	DeleteVersion(db, id2)

	ver, err := GetLatestNonPrerelease(db, pkgID)
	if err != nil {
		t.Fatalf("GetLatestNonPrerelease: %v", err)
	}
	if ver != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0 after delete", ver)
	}
}

func TestInitDB_BadPath(t *testing.T) {
	_, err := InitDB("/nonexistent/path/that/should/fail/db.sqlite")
	if err == nil {
		t.Error("InitDB with bad path should fail")
	}
}

func TestGetPackageByID_NotFound(t *testing.T) {
	db, _ := TestDB()
	defer db.Close()
	_, err := GetPackageByID(db, 99999)
	if err != sql.ErrNoRows {
		t.Errorf("err = %v, want sql.ErrNoRows", err)
	}
}

func TestSearchPackages_Empty(t *testing.T) {
	db, _ := TestDB()
	defer db.Close()

	r, err := SearchPackages(db, "nonexistent", "", 1, 10)
	if err != nil {
		t.Fatalf("SearchPackages: %v", err)
	}
	if r.Total != 0 {
		t.Errorf("Total = %d, want 0", r.Total)
	}
	if len(r.Packages) != 0 {
		t.Errorf("len = %d, want 0", len(r.Packages))
	}
}

func TestListPackages_EmptyType(t *testing.T) {
	db, _ := TestDB()
	defer db.Close()

	r, err := ListPackages(db, "assistant", "", "", 1, 10)
	if err != nil {
		t.Fatalf("ListPackages: %v", err)
	}
	if r.Total != 0 {
		t.Errorf("Total = %d, want 0", r.Total)
	}
}

func TestListPackages_InvalidPageDefaults(t *testing.T) {
	db, _ := TestDB()
	defer db.Close()

	UpsertPackage(db, &Package{
		Type: "assistant", Scope: "@yao", Name: "test",
		Keywords: "[]", DistTags: "{}",
	})

	r, err := ListPackages(db, "assistant", "", "", 0, -1)
	if err != nil {
		t.Fatalf("ListPackages: %v", err)
	}
	if r.Total != 1 {
		t.Errorf("Total = %d, want 1", r.Total)
	}
}

func TestUpsertPackage_ThreeTimesReturnsSameID(t *testing.T) {
	db, _ := TestDB()
	defer db.Close()

	pkg := &Package{
		Type: "assistant", Scope: "@yao", Name: "keeper",
		Keywords: "[]", DistTags: "{}",
	}

	id1, _ := UpsertPackage(db, pkg)
	id2, _ := UpsertPackage(db, pkg)
	id3, _ := UpsertPackage(db, pkg)

	if id1 != id2 || id2 != id3 {
		t.Errorf("IDs differ: %d, %d, %d", id1, id2, id3)
	}
}

func TestDeleteDependenciesByVersion_Cascade(t *testing.T) {
	db, _ := TestDB()
	defer db.Close()

	_, verID := setupVersionForDep(t, db, "assistant", "@yao", "keeper", "1.0.0")

	InsertDependencies(db, verID, []Dependency{
		{DepType: "mcp", DepScope: "@yao", DepName: "a", DepVersion: "^1"},
		{DepType: "mcp", DepScope: "@yao", DepName: "b", DepVersion: "^2"},
		{DepType: "assistant", DepScope: "@yao", DepName: "c", DepVersion: "~1"},
	})

	deps, _ := GetDependencies(db, verID)
	if len(deps) != 3 {
		t.Fatalf("before: len = %d", len(deps))
	}

	DeleteDependenciesByVersion(db, verID)

	deps, _ = GetDependencies(db, verID)
	if len(deps) != 0 {
		t.Errorf("after: len = %d", len(deps))
	}
}
