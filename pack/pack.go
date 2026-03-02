// Package pack handles .yao.zip archive parsing, including JSONC-aware
// pkg.yao manifest extraction and validation.
package pack

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrNoPkgYao        = errors.New("package/pkg.yao not found in archive")
	ErrInvalidPkgYao   = errors.New("invalid pkg.yao content")
	ErrTypeMismatch    = errors.New("pkg.yao type does not match URL")
	ErrScopeMismatch   = errors.New("pkg.yao scope does not match URL")
	ErrNameMismatch    = errors.New("pkg.yao name does not match URL")
	ErrVersionMismatch = errors.New("pkg.yao version does not match URL")
)

// PkgDependency represents a single dependency declaration in pkg.yao.
type PkgDependency struct {
	Type    string `json:"type"`
	Scope   string `json:"scope"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

// PkgPlatform describes a platform-specific entry for Release packages.
type PkgPlatform struct {
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	Variant string `json:"variant,omitempty"`
}

// PersonInfo represents author or maintainer contact info.
type PersonInfo struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

// RepoInfo represents repository reference info.
type RepoInfo struct {
	Type string `json:"type,omitempty"`
	URL  string `json:"url,omitempty"`
}

// BugsInfo represents bug tracking info.
type BugsInfo struct {
	URL string `json:"url,omitempty"`
}

// PkgYao represents the parsed content of a pkg.yao manifest file.
type PkgYao struct {
	Type         string                 `json:"type"`
	Scope        string                 `json:"scope"`
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	Description  string                 `json:"description,omitempty"`
	Keywords     []string               `json:"keywords,omitempty"`
	Icon         string                 `json:"icon,omitempty"`
	License      string                 `json:"license,omitempty"`
	Author       *PersonInfo            `json:"author,omitempty"`
	Maintainers  []PersonInfo           `json:"maintainers,omitempty"`
	Homepage     string                 `json:"homepage,omitempty"`
	Repository   *RepoInfo              `json:"repository,omitempty"`
	Bugs         *BugsInfo              `json:"bugs,omitempty"`
	Engines      map[string]string      `json:"engines,omitempty"`
	Dependencies []PkgDependency        `json:"dependencies,omitempty"`
	Platform     *PkgPlatform           `json:"platform,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ExtractPkgYao reads and parses package/pkg.yao from a .yao.zip archive.
func ExtractPkgYao(zipData []byte) (*PkgYao, error) {
	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}

	for _, f := range r.File {
		if f.Name == "package/pkg.yao" {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("open pkg.yao: %w", err)
			}
			defer rc.Close()

			buf := new(bytes.Buffer)
			if _, err := buf.ReadFrom(rc); err != nil {
				return nil, fmt.Errorf("read pkg.yao: %w", err)
			}

			cleaned := StripJSONComments(buf.Bytes())
			var pkg PkgYao
			if err := json.Unmarshal(cleaned, &pkg); err != nil {
				return nil, fmt.Errorf("%w: %v", ErrInvalidPkgYao, err)
			}
			return &pkg, nil
		}
	}
	return nil, ErrNoPkgYao
}

// ExtractReadme reads package/README.md (case-insensitive) from a .yao.zip.
func ExtractReadme(zipData []byte) (string, error) {
	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return "", fmt.Errorf("open zip: %w", err)
	}

	for _, f := range r.File {
		lower := strings.ToLower(f.Name)
		if lower == "package/readme.md" {
			rc, err := f.Open()
			if err != nil {
				return "", fmt.Errorf("open readme: %w", err)
			}
			defer rc.Close()

			buf := new(bytes.Buffer)
			buf.ReadFrom(rc)
			return buf.String(), nil
		}
	}
	return "", nil // no README is not an error
}

// ValidatePkgYao checks that pkg.yao fields match the URL parameters.
func ValidatePkgYao(pkg *PkgYao, urlType, urlScope, urlName, urlVersion string) error {
	if pkg.Type != urlType {
		return fmt.Errorf("%w: pkg.yao=%q, url=%q", ErrTypeMismatch, pkg.Type, urlType)
	}
	if pkg.Scope != urlScope {
		return fmt.Errorf("%w: pkg.yao=%q, url=%q", ErrScopeMismatch, pkg.Scope, urlScope)
	}
	if pkg.Name != urlName {
		return fmt.Errorf("%w: pkg.yao=%q, url=%q", ErrNameMismatch, pkg.Name, urlName)
	}
	if pkg.Version != urlVersion {
		return fmt.Errorf("%w: pkg.yao=%q, url=%q", ErrVersionMismatch, pkg.Version, urlVersion)
	}
	return nil
}

// StripJSONComments removes // line comments and /* block comments */ from
// JSONC content, producing valid JSON.
func StripJSONComments(data []byte) []byte {
	var out bytes.Buffer
	i := 0
	n := len(data)
	inString := false

	for i < n {
		ch := data[i]

		if inString {
			out.WriteByte(ch)
			if ch == '\\' && i+1 < n {
				i++
				out.WriteByte(data[i])
			} else if ch == '"' {
				inString = false
			}
			i++
			continue
		}

		if ch == '"' {
			inString = true
			out.WriteByte(ch)
			i++
			continue
		}

		if ch == '/' && i+1 < n {
			next := data[i+1]
			if next == '/' {
				// Skip to end of line
				i += 2
				for i < n && data[i] != '\n' {
					i++
				}
				continue
			}
			if next == '*' {
				// Skip to closing */
				i += 2
				for i+1 < n {
					if data[i] == '*' && data[i+1] == '/' {
						i += 2
						break
					}
					i++
				}
				continue
			}
		}

		out.WriteByte(ch)
		i++
	}
	return out.Bytes()
}

// CreateTestZip builds a .yao.zip archive for testing with the given pkg.yao
// and additional files. File paths should be relative to the zip root
// (e.g., "package/pkg.yao").
func CreateTestZip(pkgYao *PkgYao, extraFiles map[string][]byte) ([]byte, error) {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	if pkgYao != nil {
		data, err := json.Marshal(pkgYao)
		if err != nil {
			return nil, err
		}
		f, err := w.Create("package/pkg.yao")
		if err != nil {
			return nil, err
		}
		f.Write(data)
	}

	for name, content := range extraFiles {
		f, err := w.Create(name)
		if err != nil {
			return nil, err
		}
		f.Write(content)
	}

	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
