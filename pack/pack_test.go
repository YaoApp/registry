package pack

import (
	"encoding/json"
	"errors"
	"testing"
)

func validPkgYao() *PkgYao {
	return &PkgYao{
		Type:        "assistant",
		Scope:       "@yao",
		Name:        "keeper",
		Version:     "1.0.0",
		Description: "Knowledge keeper",
		Keywords:    []string{"knowledge", "keeper"},
		License:     "Apache-2.0",
		Author:      &PersonInfo{Name: "Yao Team", Email: "team@yao.run"},
		Engines:     map[string]string{"yao": ">=2.0.0"},
		Dependencies: []PkgDependency{
			{Type: "mcp", Scope: "@yao", Name: "keeper-tools", Version: "^1.0.0"},
		},
	}
}

func TestExtractPkgYao_Valid(t *testing.T) {
	zipData, err := CreateTestZip(validPkgYao(), nil)
	if err != nil {
		t.Fatalf("CreateTestZip: %v", err)
	}

	pkg, err := ExtractPkgYao(zipData)
	if err != nil {
		t.Fatalf("ExtractPkgYao: %v", err)
	}
	if pkg.Type != "assistant" {
		t.Errorf("Type = %q", pkg.Type)
	}
	if pkg.Scope != "@yao" {
		t.Errorf("Scope = %q", pkg.Scope)
	}
	if pkg.Name != "keeper" {
		t.Errorf("Name = %q", pkg.Name)
	}
	if pkg.Version != "1.0.0" {
		t.Errorf("Version = %q", pkg.Version)
	}
	if pkg.License != "Apache-2.0" {
		t.Errorf("License = %q", pkg.License)
	}
	if pkg.Author == nil || pkg.Author.Name != "Yao Team" {
		t.Errorf("Author = %+v", pkg.Author)
	}
	if len(pkg.Dependencies) != 1 {
		t.Fatalf("Dependencies len = %d", len(pkg.Dependencies))
	}
	if pkg.Dependencies[0].Name != "keeper-tools" {
		t.Errorf("dep name = %q", pkg.Dependencies[0].Name)
	}
	if pkg.Engines["yao"] != ">=2.0.0" {
		t.Errorf("engines = %v", pkg.Engines)
	}
}

func TestExtractPkgYao_NoPkgYao(t *testing.T) {
	zipData, _ := CreateTestZip(nil, map[string][]byte{
		"package/other.txt": []byte("hello"),
	})

	_, err := ExtractPkgYao(zipData)
	if !errors.Is(err, ErrNoPkgYao) {
		t.Errorf("err = %v, want ErrNoPkgYao", err)
	}
}

func TestExtractPkgYao_InvalidJSON(t *testing.T) {
	zipData, _ := CreateTestZip(nil, map[string][]byte{
		"package/pkg.yao": []byte("{invalid json}"),
	})

	_, err := ExtractPkgYao(zipData)
	if !errors.Is(err, ErrInvalidPkgYao) {
		t.Errorf("err = %v, want ErrInvalidPkgYao", err)
	}
}

func TestExtractPkgYao_InvalidZip(t *testing.T) {
	_, err := ExtractPkgYao([]byte("not a zip"))
	if err == nil {
		t.Error("expected error for invalid zip")
	}
}

func TestExtractReadme(t *testing.T) {
	readmeContent := "# Keeper\nA knowledge management assistant."
	zipData, _ := CreateTestZip(validPkgYao(), map[string][]byte{
		"package/README.md": []byte(readmeContent),
	})

	readme, err := ExtractReadme(zipData)
	if err != nil {
		t.Fatalf("ExtractReadme: %v", err)
	}
	if readme != readmeContent {
		t.Errorf("readme = %q", readme)
	}
}

func TestExtractReadme_CaseInsensitive(t *testing.T) {
	zipData, _ := CreateTestZip(validPkgYao(), map[string][]byte{
		"package/readme.md": []byte("lowercase readme"),
	})

	readme, _ := ExtractReadme(zipData)
	if readme != "lowercase readme" {
		t.Errorf("readme = %q", readme)
	}
}

func TestExtractReadme_MixedCase(t *testing.T) {
	zipData, _ := CreateTestZip(validPkgYao(), map[string][]byte{
		"package/Readme.MD": []byte("mixed case"),
	})

	readme, _ := ExtractReadme(zipData)
	if readme != "mixed case" {
		t.Errorf("readme = %q", readme)
	}
}

func TestExtractReadme_Missing(t *testing.T) {
	zipData, _ := CreateTestZip(validPkgYao(), nil)
	readme, err := ExtractReadme(zipData)
	if err != nil {
		t.Fatalf("ExtractReadme: %v", err)
	}
	if readme != "" {
		t.Errorf("readme = %q, want empty", readme)
	}
}

func TestValidatePkgYao_Valid(t *testing.T) {
	pkg := validPkgYao()
	err := ValidatePkgYao(pkg, "assistant", "@yao", "keeper", "1.0.0")
	if err != nil {
		t.Errorf("ValidatePkgYao: %v", err)
	}
}

func TestValidatePkgYao_TypeMismatch(t *testing.T) {
	pkg := validPkgYao()
	err := ValidatePkgYao(pkg, "mcp", "@yao", "keeper", "1.0.0")
	if !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("err = %v, want ErrTypeMismatch", err)
	}
}

func TestValidatePkgYao_ScopeMismatch(t *testing.T) {
	pkg := validPkgYao()
	err := ValidatePkgYao(pkg, "assistant", "@community", "keeper", "1.0.0")
	if !errors.Is(err, ErrScopeMismatch) {
		t.Errorf("err = %v, want ErrScopeMismatch", err)
	}
}

func TestValidatePkgYao_NameMismatch(t *testing.T) {
	pkg := validPkgYao()
	err := ValidatePkgYao(pkg, "assistant", "@yao", "different", "1.0.0")
	if !errors.Is(err, ErrNameMismatch) {
		t.Errorf("err = %v, want ErrNameMismatch", err)
	}
}

func TestValidatePkgYao_VersionMismatch(t *testing.T) {
	pkg := validPkgYao()
	err := ValidatePkgYao(pkg, "assistant", "@yao", "keeper", "2.0.0")
	if !errors.Is(err, ErrVersionMismatch) {
		t.Errorf("err = %v, want ErrVersionMismatch", err)
	}
}

func TestStripJSONComments_LineComments(t *testing.T) {
	input := []byte(`{
  // this is a comment
  "name": "test" // inline comment
}`)
	got := StripJSONComments(input)
	var m map[string]string
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("parse after strip: %v\ngot: %s", err, got)
	}
	if m["name"] != "test" {
		t.Errorf("name = %q", m["name"])
	}
}

func TestStripJSONComments_BlockComments(t *testing.T) {
	input := []byte(`{
  /* block comment */
  "key": /* mid */ "value"
}`)
	got := StripJSONComments(input)
	var m map[string]string
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("parse after strip: %v\ngot: %s", err, got)
	}
	if m["key"] != "value" {
		t.Errorf("key = %q", m["key"])
	}
}

func TestStripJSONComments_InString(t *testing.T) {
	input := []byte(`{"url": "https://example.com/path"}`)
	got := StripJSONComments(input)
	var m map[string]string
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if m["url"] != "https://example.com/path" {
		t.Errorf("url = %q", m["url"])
	}
}

func TestStripJSONComments_EscapedQuoteInString(t *testing.T) {
	input := []byte(`{"msg": "say \"hello\""}`)
	got := StripJSONComments(input)
	var m map[string]string
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if m["msg"] != `say "hello"` {
		t.Errorf("msg = %q", m["msg"])
	}
}

func TestStripJSONComments_NoComments(t *testing.T) {
	input := []byte(`{"a": 1, "b": "two"}`)
	got := StripJSONComments(input)
	if string(got) != string(input) {
		t.Errorf("output changed: %s", got)
	}
}

func TestCreateTestZip(t *testing.T) {
	pkg := validPkgYao()
	data, err := CreateTestZip(pkg, map[string][]byte{
		"package/README.md":    []byte("# Test"),
		"package/scripts/a.js": []byte("console.log('hi')"),
	})
	if err != nil {
		t.Fatalf("CreateTestZip: %v", err)
	}
	if len(data) == 0 {
		t.Error("zip data is empty")
	}

	// Verify we can extract pkg.yao from it
	extracted, err := ExtractPkgYao(data)
	if err != nil {
		t.Fatalf("ExtractPkgYao: %v", err)
	}
	if extracted.Name != "keeper" {
		t.Errorf("Name = %q", extracted.Name)
	}

	// Verify README
	readme, _ := ExtractReadme(data)
	if readme != "# Test" {
		t.Errorf("readme = %q", readme)
	}
}
