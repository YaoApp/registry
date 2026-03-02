package models

import (
	"database/sql"
	"fmt"
	"testing"
)

func testPkg() *Package {
	return &Package{
		Type:        "assistant",
		Scope:       "@yao",
		Name:        "keeper",
		Description: "Knowledge keeper assistant",
		Keywords:    `["knowledge","keeper"]`,
		License:     "Apache-2.0",
		Author:      `{"name":"Yao Team"}`,
		Maintainers: `[{"name":"admin"}]`,
		DistTags:    `{}`,
	}
}

func TestUpsertPackage_Insert(t *testing.T) {
	db, err := TestDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	tx, _ := db.Begin()
	defer tx.Rollback()

	pkg := testPkg()
	id, err := UpsertPackage(tx, pkg)
	if err != nil {
		t.Fatalf("UpsertPackage: %v", err)
	}
	if id < 1 {
		t.Errorf("id = %d, want > 0", id)
	}
	tx.Commit()

	got, err := GetPackageByID(db, id)
	if err != nil {
		t.Fatalf("GetPackageByID: %v", err)
	}
	if got.Name != "keeper" {
		t.Errorf("Name = %q, want %q", got.Name, "keeper")
	}
	if got.Description != "Knowledge keeper assistant" {
		t.Errorf("Description = %q", got.Description)
	}
	if got.License != "Apache-2.0" {
		t.Errorf("License = %q", got.License)
	}
}

func TestUpsertPackage_Update(t *testing.T) {
	db, err := TestDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	pkg := testPkg()
	tx1, _ := db.Begin()
	id1, _ := UpsertPackage(tx1, pkg)
	tx1.Commit()

	pkg.Description = "Updated description"
	tx2, _ := db.Begin()
	id2, err := UpsertPackage(tx2, pkg)
	if err != nil {
		t.Fatalf("upsert update: %v", err)
	}
	tx2.Commit()

	if id2 != id1 {
		t.Errorf("id changed from %d to %d on upsert", id1, id2)
	}
	got, _ := GetPackageByID(db, id1)
	if got.Description != "Updated description" {
		t.Errorf("Description = %q, want updated", got.Description)
	}
}

func TestGetPackage(t *testing.T) {
	db, err := TestDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	tx, _ := db.Begin()
	UpsertPackage(tx, testPkg())
	tx.Commit()

	got, err := GetPackage(db, "assistant", "@yao", "keeper")
	if err != nil {
		t.Fatalf("GetPackage: %v", err)
	}
	if got.Scope != "@yao" {
		t.Errorf("Scope = %q, want @yao", got.Scope)
	}
}

func TestGetPackage_NotFound(t *testing.T) {
	db, err := TestDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = GetPackage(db, "assistant", "@yao", "nonexistent")
	if err != sql.ErrNoRows {
		t.Errorf("err = %v, want sql.ErrNoRows", err)
	}
}

func TestListPackages(t *testing.T) {
	db, err := TestDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, name := range []string{"alpha", "beta", "gamma"} {
		tx, _ := db.Begin()
		UpsertPackage(tx, &Package{
			Type: "assistant", Scope: "@yao", Name: name,
			Description: name + " assistant", Keywords: "[]", DistTags: "{}",
		})
		tx.Commit()
	}

	result, err := ListPackages(db, "assistant", "", "", 1, 10)
	if err != nil {
		t.Fatalf("ListPackages: %v", err)
	}
	if result.Total != 3 {
		t.Errorf("Total = %d, want 3", result.Total)
	}
	if len(result.Packages) != 3 {
		t.Errorf("len = %d, want 3", len(result.Packages))
	}
}

func TestListPackages_Pagination(t *testing.T) {
	db, err := TestDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for i := 0; i < 5; i++ {
		tx, _ := db.Begin()
		UpsertPackage(tx, &Package{
			Type: "mcp", Scope: "@yao", Name: fmt.Sprintf("tool-%d", i),
			Keywords: "[]", DistTags: "{}",
		})
		tx.Commit()
	}

	r, _ := ListPackages(db, "mcp", "", "", 1, 2)
	if r.Total != 5 {
		t.Errorf("Total = %d, want 5", r.Total)
	}
	if len(r.Packages) != 2 {
		t.Errorf("page 1 len = %d, want 2", len(r.Packages))
	}

	r2, _ := ListPackages(db, "mcp", "", "", 3, 2)
	if len(r2.Packages) != 1 {
		t.Errorf("page 3 len = %d, want 1", len(r2.Packages))
	}
}

func TestListPackages_FilterScope(t *testing.T) {
	db, err := TestDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	tx, _ := db.Begin()
	UpsertPackage(tx, &Package{Type: "assistant", Scope: "@yao", Name: "a", Keywords: "[]", DistTags: "{}"})
	UpsertPackage(tx, &Package{Type: "assistant", Scope: "@community", Name: "b", Keywords: "[]", DistTags: "{}"})
	tx.Commit()

	r, _ := ListPackages(db, "assistant", "@yao", "", 1, 10)
	if r.Total != 1 {
		t.Errorf("Total = %d, want 1", r.Total)
	}
}

func TestListPackages_Search(t *testing.T) {
	db, err := TestDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	tx, _ := db.Begin()
	UpsertPackage(tx, &Package{Type: "assistant", Scope: "@yao", Name: "keeper",
		Description: "Knowledge keeper", Keywords: `["knowledge"]`, DistTags: "{}"})
	UpsertPackage(tx, &Package{Type: "assistant", Scope: "@yao", Name: "translator",
		Description: "Translation tool", Keywords: `["i18n"]`, DistTags: "{}"})
	tx.Commit()

	r, _ := ListPackages(db, "assistant", "", "knowledge", 1, 10)
	if r.Total != 1 {
		t.Errorf("Total = %d, want 1", r.Total)
	}
	if r.Packages[0].Name != "keeper" {
		t.Errorf("Name = %q, want keeper", r.Packages[0].Name)
	}
}

func TestSearchPackages(t *testing.T) {
	db, err := TestDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	tx, _ := db.Begin()
	UpsertPackage(tx, &Package{Type: "assistant", Scope: "@yao", Name: "keeper",
		Description: "Data management", Keywords: "[]", DistTags: "{}"})
	UpsertPackage(tx, &Package{Type: "mcp", Scope: "@yao", Name: "data-tools",
		Description: "Data processing tools", Keywords: "[]", DistTags: "{}"})
	UpsertPackage(tx, &Package{Type: "robot", Scope: "@yao", Name: "bot",
		Description: "A simple bot", Keywords: "[]", DistTags: "{}"})
	tx.Commit()

	r, _ := SearchPackages(db, "data", "", 1, 10)
	if r.Total != 2 {
		t.Errorf("Total = %d, want 2 (cross-type search)", r.Total)
	}

	r2, _ := SearchPackages(db, "data", "mcp", 1, 10)
	if r2.Total != 1 {
		t.Errorf("filtered Total = %d, want 1", r2.Total)
	}
}

func TestUpdateDistTags(t *testing.T) {
	db, err := TestDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	tx, _ := db.Begin()
	id, _ := UpsertPackage(tx, testPkg())
	tx.Commit()

	tx2, _ := db.Begin()
	err = UpdateDistTags(tx2, id, `{"latest":"1.0.0","canary":"1.1.0-beta"}`)
	if err != nil {
		t.Fatalf("UpdateDistTags: %v", err)
	}
	tx2.Commit()

	got, _ := GetPackageByID(db, id)
	if got.DistTags != `{"latest":"1.0.0","canary":"1.1.0-beta"}` {
		t.Errorf("DistTags = %q", got.DistTags)
	}
}
