package models

import (
	"database/sql"
	"testing"
)

func TestGetLatestNonPrereleaseTx(t *testing.T) {
	db, _ := TestDB()
	defer db.Close()

	tx, _ := db.Begin()
	pkgID, _ := UpsertPackage(tx, &Package{
		Type: "assistant", Scope: "@yao", Name: "keeper",
		Keywords: "[]", DistTags: "{}",
	})
	InsertVersion(tx, &Version{
		PackageID: pkgID, Version: "1.0.0",
		Digest: "sha256:a", Size: 100, Metadata: "{}", FilePath: "a.zip",
	})
	InsertVersion(tx, &Version{
		PackageID: pkgID, Version: "2.0.0-beta",
		Digest: "sha256:b", Size: 200, Metadata: "{}", FilePath: "b.zip",
	})
	InsertVersion(tx, &Version{
		PackageID: pkgID, Version: "1.1.0",
		Digest: "sha256:c", Size: 300, Metadata: "{}", FilePath: "c.zip",
	})
	tx.Commit()

	tx2, _ := db.Begin()
	defer tx2.Rollback()
	ver, err := GetLatestNonPrereleaseTx(tx2, pkgID)
	if err != nil {
		t.Fatalf("GetLatestNonPrereleaseTx: %v", err)
	}
	if ver != "1.1.0" {
		t.Errorf("version = %q, want 1.1.0", ver)
	}
}

func TestGetLatestNonPrereleaseTx_AfterDelete(t *testing.T) {
	db, _ := TestDB()
	defer db.Close()

	tx, _ := db.Begin()
	pkgID, _ := UpsertPackage(tx, &Package{
		Type: "assistant", Scope: "@yao", Name: "keeper",
		Keywords: "[]", DistTags: "{}",
	})
	id1, _ := InsertVersion(tx, &Version{
		PackageID: pkgID, Version: "1.0.0",
		Digest: "sha256:a", Size: 100, Metadata: "{}", FilePath: "a.zip",
	})
	InsertVersion(tx, &Version{
		PackageID: pkgID, Version: "1.1.0",
		Digest: "sha256:b", Size: 200, Metadata: "{}", FilePath: "b.zip",
	})
	tx.Commit()

	tx2, _ := db.Begin()
	// Delete 1.1.0's predecessor
	DeleteVersion(tx2, id1+1) // id of 1.1.0

	ver, err := GetLatestNonPrereleaseTx(tx2, pkgID)
	if err != nil {
		t.Fatalf("GetLatestNonPrereleaseTx: %v", err)
	}
	if ver != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0 after delete", ver)
	}
	tx2.Commit()
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

	tx, _ := db.Begin()
	UpsertPackage(tx, &Package{
		Type: "assistant", Scope: "@yao", Name: "test",
		Keywords: "[]", DistTags: "{}",
	})
	tx.Commit()

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

	tx1, _ := db.Begin()
	id1, _ := UpsertPackage(tx1, pkg)
	tx1.Commit()

	tx2, _ := db.Begin()
	id2, _ := UpsertPackage(tx2, pkg)
	tx2.Commit()

	tx3, _ := db.Begin()
	id3, _ := UpsertPackage(tx3, pkg)
	tx3.Commit()

	if id1 != id2 || id2 != id3 {
		t.Errorf("IDs differ: %d, %d, %d", id1, id2, id3)
	}
}

func TestDeleteDependenciesByVersion_Cascade(t *testing.T) {
	db, _ := TestDB()
	defer db.Close()

	_, verID := setupVersionForDep(t, db, "assistant", "@yao", "keeper", "1.0.0")

	tx, _ := db.Begin()
	InsertDependencies(tx, verID, []Dependency{
		{DepType: "mcp", DepScope: "@yao", DepName: "a", DepVersion: "^1"},
		{DepType: "mcp", DepScope: "@yao", DepName: "b", DepVersion: "^2"},
		{DepType: "assistant", DepScope: "@yao", DepName: "c", DepVersion: "~1"},
	})
	tx.Commit()

	deps, _ := GetDependencies(db, verID)
	if len(deps) != 3 {
		t.Fatalf("before: len = %d", len(deps))
	}

	tx2, _ := db.Begin()
	DeleteDependenciesByVersion(tx2, verID)
	tx2.Commit()

	deps, _ = GetDependencies(db, verID)
	if len(deps) != 0 {
		t.Errorf("after: len = %d", len(deps))
	}
}
