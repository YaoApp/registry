package models

import (
	"database/sql"
	"testing"
)

func setupPkgForVersion(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	tx, _ := db.Begin()
	id, err := UpsertPackage(tx, &Package{
		Type: "assistant", Scope: "@yao", Name: "keeper",
		Keywords: "[]", DistTags: "{}",
	})
	if err != nil {
		t.Fatalf("setup package: %v", err)
	}
	tx.Commit()
	return id
}

func TestInsertVersion(t *testing.T) {
	db, _ := TestDB()
	defer db.Close()
	pkgID := setupPkgForVersion(t, db)

	tx, _ := db.Begin()
	id, err := InsertVersion(tx, &Version{
		PackageID: pkgID, Version: "1.0.0",
		Digest: "sha256:abc", Size: 1024,
		Metadata: `{"modes":["chat"]}`, FilePath: "assistants/@yao/keeper/1.0.0.yao.zip",
	})
	tx.Commit()

	if err != nil {
		t.Fatalf("InsertVersion: %v", err)
	}
	if id < 1 {
		t.Errorf("id = %d, want > 0", id)
	}

	got, err := GetVersionByID(db, id)
	if err != nil {
		t.Fatalf("GetVersionByID: %v", err)
	}
	if got.Version != "1.0.0" {
		t.Errorf("Version = %q", got.Version)
	}
	if got.Digest != "sha256:abc" {
		t.Errorf("Digest = %q", got.Digest)
	}
	if got.Size != 1024 {
		t.Errorf("Size = %d", got.Size)
	}
}

func TestInsertVersion_DuplicateRejected(t *testing.T) {
	db, _ := TestDB()
	defer db.Close()
	pkgID := setupPkgForVersion(t, db)

	v := &Version{
		PackageID: pkgID, Version: "1.0.0",
		Digest: "sha256:abc", Size: 1024,
		Metadata: "{}", FilePath: "path.zip",
	}

	tx, _ := db.Begin()
	InsertVersion(tx, v)
	tx.Commit()

	tx2, _ := db.Begin()
	_, err := InsertVersion(tx2, v)
	tx2.Rollback()

	if err == nil {
		t.Error("duplicate insert should fail")
	}
}

func TestGetVersion(t *testing.T) {
	db, _ := TestDB()
	defer db.Close()
	pkgID := setupPkgForVersion(t, db)

	tx, _ := db.Begin()
	InsertVersion(tx, &Version{
		PackageID: pkgID, Version: "2.0.0",
		Digest: "sha256:def", Size: 2048,
		Metadata: "{}", FilePath: "path.zip",
	})
	tx.Commit()

	got, err := GetVersion(db, pkgID, "2.0.0", "", "", "")
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	if got.Version != "2.0.0" {
		t.Errorf("Version = %q", got.Version)
	}
}

func TestGetVersion_NotFound(t *testing.T) {
	db, _ := TestDB()
	defer db.Close()
	pkgID := setupPkgForVersion(t, db)

	_, err := GetVersion(db, pkgID, "9.9.9", "", "", "")
	if err != sql.ErrNoRows {
		t.Errorf("err = %v, want sql.ErrNoRows", err)
	}
}

func TestListVersions(t *testing.T) {
	db, _ := TestDB()
	defer db.Close()
	pkgID := setupPkgForVersion(t, db)

	for _, ver := range []string{"1.0.0", "1.1.0", "2.0.0"} {
		tx, _ := db.Begin()
		InsertVersion(tx, &Version{
			PackageID: pkgID, Version: ver,
			Digest: "sha256:" + ver, Size: 1024,
			Metadata: "{}", FilePath: ver + ".zip",
		})
		tx.Commit()
	}

	versions, err := ListVersions(db, pkgID)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 3 {
		t.Errorf("len = %d, want 3", len(versions))
	}
}

func TestDeleteVersion(t *testing.T) {
	db, _ := TestDB()
	defer db.Close()
	pkgID := setupPkgForVersion(t, db)

	tx, _ := db.Begin()
	id, _ := InsertVersion(tx, &Version{
		PackageID: pkgID, Version: "1.0.0",
		Digest: "sha256:abc", Size: 1024,
		Metadata: "{}", FilePath: "path.zip",
	})
	tx.Commit()

	tx2, _ := db.Begin()
	err := DeleteVersion(tx2, id)
	if err != nil {
		t.Fatalf("DeleteVersion: %v", err)
	}
	tx2.Commit()

	_, err = GetVersionByID(db, id)
	if err != sql.ErrNoRows {
		t.Errorf("after delete, err = %v, want sql.ErrNoRows", err)
	}
}

func TestCountVersions(t *testing.T) {
	db, _ := TestDB()
	defer db.Close()
	pkgID := setupPkgForVersion(t, db)

	count, _ := CountVersions(db, pkgID)
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}

	tx, _ := db.Begin()
	InsertVersion(tx, &Version{
		PackageID: pkgID, Version: "1.0.0",
		Digest: "sha256:a", Size: 100, Metadata: "{}", FilePath: "a.zip",
	})
	InsertVersion(tx, &Version{
		PackageID: pkgID, Version: "1.1.0",
		Digest: "sha256:b", Size: 200, Metadata: "{}", FilePath: "b.zip",
	})
	tx.Commit()

	count, _ = CountVersions(db, pkgID)
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestGetLatestNonPrerelease(t *testing.T) {
	db, _ := TestDB()
	defer db.Close()
	pkgID := setupPkgForVersion(t, db)

	tx, _ := db.Begin()
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

	ver, err := GetLatestNonPrerelease(db, pkgID)
	if err != nil {
		t.Fatalf("GetLatestNonPrerelease: %v", err)
	}
	if ver != "1.1.0" {
		t.Errorf("version = %q, want 1.1.0", ver)
	}
}

func TestListVersionsByPlatform(t *testing.T) {
	db, _ := TestDB()
	defer db.Close()
	pkgID := setupPkgForVersion(t, db)

	tx, _ := db.Begin()
	InsertVersion(tx, &Version{
		PackageID: pkgID, Version: "1.0.0",
		OS: "linux", Arch: "amd64", Variant: "prod",
		Digest: "sha256:a", Size: 100, Metadata: "{}", FilePath: "a.zip",
	})
	InsertVersion(tx, &Version{
		PackageID: pkgID, Version: "1.0.0",
		OS: "darwin", Arch: "arm64", Variant: "prod",
		Digest: "sha256:b", Size: 200, Metadata: "{}", FilePath: "b.zip",
	})
	InsertVersion(tx, &Version{
		PackageID: pkgID, Version: "1.0.0",
		OS: "linux", Arch: "arm64", Variant: "prod",
		Digest: "sha256:c", Size: 300, Metadata: "{}", FilePath: "c.zip",
	})
	tx.Commit()

	vv, _ := ListVersionsByPlatform(db, pkgID, "1.0.0", "linux", "", "")
	if len(vv) != 2 {
		t.Errorf("linux filter: len = %d, want 2", len(vv))
	}

	vv2, _ := ListVersionsByPlatform(db, pkgID, "1.0.0", "darwin", "arm64", "")
	if len(vv2) != 1 {
		t.Errorf("darwin+arm64 filter: len = %d, want 1", len(vv2))
	}
}
