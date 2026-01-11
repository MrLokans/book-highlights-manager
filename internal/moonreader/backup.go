package moonreader

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// DBManifestLocator is the filename we look for in the manifest
	DBManifestLocator = "mrbooks.db"
	// OutputDatabaseFile is the name of the extracted database file
	OutputDatabaseFile = "exported.db"
	// MoonReaderPackage is the Android package name for MoonReader
	MoonReaderPackage = "com.flyersoft.moonreader"
)

// BackupInfo contains information about a MoonReader backup file
type BackupInfo struct {
	Path      string
	ModTime   time.Time
	Timestamp string // Timestamp from filename
}

// BackupExtractor handles extraction of MoonReader backup files
type BackupExtractor struct {
	LookupDir string
}

// NewBackupExtractor creates a new BackupExtractor with the given lookup directory
func NewBackupExtractor(lookupDir string) *BackupExtractor {
	return &BackupExtractor{LookupDir: lookupDir}
}

// NewDefaultBackupExtractor creates a BackupExtractor with the default MoonReader backup location
func NewDefaultBackupExtractor() *BackupExtractor {
	homeDir, _ := os.UserHomeDir()
	defaultPath := filepath.Join(homeDir, "syncthing", "one-plus", "moonreader", "Backup")
	return &BackupExtractor{LookupDir: defaultPath}
}

// FindLatestBackup finds the most recent MoonReader backup file
func (e *BackupExtractor) FindLatestBackup() (*BackupInfo, error) {
	if _, err := os.Stat(e.LookupDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("lookup directory does not exist: %s", e.LookupDir)
	}

	entries, err := os.ReadDir(e.LookupDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read lookup directory: %w", err)
	}

	var backupFiles []BackupInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".mrstd") || strings.HasSuffix(name, ".mrpro") {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			// Extract timestamp from filename (format: YYYYMMDD_HHMMSS.extension)
			timestamp := strings.Split(name, ".")[0]
			backupFiles = append(backupFiles, BackupInfo{
				Path:      filepath.Join(e.LookupDir, name),
				ModTime:   info.ModTime(),
				Timestamp: timestamp,
			})
		}
	}

	if len(backupFiles) == 0 {
		return nil, fmt.Errorf("no backup files found in %s", e.LookupDir)
	}

	// Sort by timestamp (filename) descending to get the latest
	sort.Slice(backupFiles, func(i, j int) bool {
		return backupFiles[i].Timestamp > backupFiles[j].Timestamp
	})

	return &backupFiles[0], nil
}

// ExtractDatabase extracts the notes database from a backup file to a temporary directory.
// Returns the path to the extracted database file.
// The caller is responsible for cleaning up the temporary directory.
func (e *BackupExtractor) ExtractDatabase(backupPath string) (dbPath string, tempDir string, err error) {
	// Create temporary directory
	tempDir, err = os.MkdirTemp("", "moonreader-backup-*")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Open the zip file
	zipReader, err := zip.OpenReader(backupPath)
	if err != nil {
		os.RemoveAll(tempDir)
		return "", "", fmt.Errorf("failed to open backup file: %w", err)
	}
	defer zipReader.Close()

	// Extract all files
	extractDir := filepath.Join(tempDir, "unzipped")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		os.RemoveAll(tempDir)
		return "", "", fmt.Errorf("failed to create extract directory: %w", err)
	}

	for _, file := range zipReader.File {
		destPath := filepath.Join(extractDir, file.Name)

		if file.FileInfo().IsDir() {
			os.MkdirAll(destPath, file.Mode())
			continue
		}

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			os.RemoveAll(tempDir)
			return "", "", fmt.Errorf("failed to create directory: %w", err)
		}

		// Extract file
		if err := extractZipFile(file, destPath); err != nil {
			os.RemoveAll(tempDir)
			return "", "", fmt.Errorf("failed to extract file %s: %w", file.Name, err)
		}
	}

	// Find the MoonReader directory (might have different suffixes like .pro)
	moonreaderDir, err := findMoonReaderDir(extractDir)
	if err != nil {
		os.RemoveAll(tempDir)
		return "", "", err
	}

	// Read the manifest file
	manifestPath := filepath.Join(moonreaderDir, "_names.list")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		os.RemoveAll(tempDir)
		return "", "", fmt.Errorf("manifest file '_names.list' does not exist")
	}

	// Find the database file reference in manifest
	dbLineNumber, err := findDBInManifest(manifestPath)
	if err != nil {
		os.RemoveAll(tempDir)
		return "", "", err
	}

	// The database file is named as {lineNumber}.tag
	presumedDBFile := filepath.Join(moonreaderDir, fmt.Sprintf("%d.tag", dbLineNumber))
	if _, err := os.Stat(presumedDBFile); os.IsNotExist(err) {
		os.RemoveAll(tempDir)
		return "", "", fmt.Errorf("presumed database file '%s' does not exist", presumedDBFile)
	}

	// Copy to output location
	outputPath := filepath.Join(moonreaderDir, OutputDatabaseFile)
	if err := copyFile(presumedDBFile, outputPath); err != nil {
		os.RemoveAll(tempDir)
		return "", "", fmt.Errorf("failed to copy database file: %w", err)
	}

	return outputPath, tempDir, nil
}

// ExtractLatestDatabase is a convenience method that finds and extracts the latest backup
func (e *BackupExtractor) ExtractLatestDatabase() (dbPath string, tempDir string, backupTime time.Time, err error) {
	backup, err := e.FindLatestBackup()
	if err != nil {
		return "", "", time.Time{}, err
	}

	dbPath, tempDir, err = e.ExtractDatabase(backup.Path)
	if err != nil {
		return "", "", time.Time{}, err
	}

	return dbPath, tempDir, backup.ModTime, nil
}

// findMoonReaderDir finds the MoonReader directory within the extracted backup
func findMoonReaderDir(extractDir string) (string, error) {
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return "", fmt.Errorf("failed to read extract directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "com.flyersoft") {
			return filepath.Join(extractDir, entry.Name()), nil
		}
	}

	return "", fmt.Errorf("MoonReader directory not found in backup")
}

// findDBInManifest finds the line number of the database file in the manifest
func findDBInManifest(manifestPath string) (int, error) {
	file, err := os.Open(manifestPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open manifest: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		// Get the filename part of the path
		parts := strings.Split(line, "/")
		filename := parts[len(parts)-1]
		if filename == DBManifestLocator || strings.Contains(filename, DBManifestLocator) {
			return lineNumber, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("error reading manifest: %w", err)
	}

	return 0, fmt.Errorf("database manifest locator '%s' not found in manifest", DBManifestLocator)
}

// extractZipFile extracts a single file from a zip archive
func extractZipFile(file *zip.File, destPath string) error {
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, rc)
	return err
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
