package demo

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed assets
var embeddedAssets embed.FS

// ExtractAssets extracts embedded demo assets to a target directory.
// Returns paths to the extracted database, covers directory, and vault directory.
func ExtractAssets(targetDir string) (dbPath, coversDir, vaultDir string, err error) {
	// Create target directory structure
	dbPath = filepath.Join(targetDir, "demo.db")
	coversDir = filepath.Join(targetDir, "covers")
	vaultDir = filepath.Join(targetDir, "vault")

	if err := os.MkdirAll(coversDir, 0755); err != nil {
		return "", "", "", fmt.Errorf("create covers dir: %w", err)
	}
	if err := os.MkdirAll(vaultDir, 0755); err != nil {
		return "", "", "", fmt.Errorf("create vault dir: %w", err)
	}

	// Extract database
	dbData, err := embeddedAssets.ReadFile("assets/demo.db")
	if err != nil {
		return "", "", "", fmt.Errorf("read embedded database: %w", err)
	}
	if err := os.WriteFile(dbPath, dbData, 0644); err != nil {
		return "", "", "", fmt.Errorf("write database: %w", err)
	}

	// Extract covers
	coversFS, err := fs.Sub(embeddedAssets, "assets/covers")
	if err != nil {
		return "", "", "", fmt.Errorf("access covers fs: %w", err)
	}

	err = fs.WalkDir(coversFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		srcFile, err := coversFS.Open(path)
		if err != nil {
			return fmt.Errorf("open cover %s: %w", path, err)
		}
		defer srcFile.Close()

		dstPath := filepath.Join(coversDir, path)
		dstFile, err := os.Create(dstPath)
		if err != nil {
			return fmt.Errorf("create cover %s: %w", path, err)
		}
		defer dstFile.Close()

		if _, err := io.Copy(dstFile, srcFile); err != nil {
			return fmt.Errorf("copy cover %s: %w", path, err)
		}

		return nil
	})
	if err != nil {
		return "", "", "", fmt.Errorf("extract covers: %w", err)
	}

	return dbPath, coversDir, vaultDir, nil
}

// HasEmbeddedAssets returns true if embedded demo assets are available.
func HasEmbeddedAssets() bool {
	_, err := embeddedAssets.ReadFile("assets/demo.db")
	return err == nil
}
