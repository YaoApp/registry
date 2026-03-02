// Package storage provides file-based storage for .yao.zip packages.
// Packages are stored in a hierarchical directory layout:
//
//	<base>/<type>/<scope>/<name>/<version>[/<os>-<arch>[-<variant>]].yao.zip
package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Store writes package data to disk and returns the relative file path.
// For non-release packages, os/arch/variant should be empty strings.
func Store(basePath, typeName, scope, name, version, goos, arch, variant string, data []byte) (string, error) {
	dir := filepath.Join(basePath, typeName, scope, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create dir: %w", err)
	}

	filename := version
	if goos != "" || arch != "" {
		parts := []string{goos, arch}
		if variant != "" {
			parts = append(parts, variant)
		}
		filename += "-" + strings.Join(parts, "-")
	}
	filename += ".yao.zip"

	relPath := filepath.Join(typeName, scope, name, filename)
	fullPath := filepath.Join(basePath, relPath)

	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}
	return relPath, nil
}

// Load opens a stored file for reading. Caller must close the returned reader.
func Load(basePath, relPath string) (io.ReadCloser, int64, error) {
	fullPath := filepath.Join(basePath, relPath)
	f, err := os.Open(fullPath)
	if err != nil {
		return nil, 0, fmt.Errorf("open file: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, fmt.Errorf("stat file: %w", err)
	}
	return f, info.Size(), nil
}

// Delete removes a stored file.
func Delete(basePath, relPath string) error {
	fullPath := filepath.Join(basePath, relPath)
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove file: %w", err)
	}
	return nil
}

// ComputeDigest returns the sha256 digest of data in "sha256:<hex>" format.
func ComputeDigest(data []byte) string {
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:])
}

// EnsureDir creates the base storage directory if it doesn't exist.
func EnsureDir(basePath string) error {
	return os.MkdirAll(basePath, 0755)
}
