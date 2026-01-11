package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

type Auditor struct {
	AuditDir string
}

func NewAuditor(auditDir string) *Auditor {
	return &Auditor{
		AuditDir: auditDir,
	}
}

// SaveJSON saves the provided data as JSON to a file with UUID4 filename
func (a *Auditor) SaveJSON(data any) (string, error) {
	// Ensure audit directory exists
	if err := a.ensureAuditDir(); err != nil {
		return "", fmt.Errorf("failed to ensure audit directory: %w", err)
	}

	// Generate UUID4 for filename
	auditID := uuid.New()
	filename := fmt.Sprintf("%s.json", auditID.String())
	filepath := filepath.Join(a.AuditDir, filename)

	fmt.Printf("Saving audit file: %s", filepath)

	// Marshal data to JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal data to JSON: %w", err)
	}

	// Write JSON to file
	if err := os.WriteFile(filepath, jsonData, 0644); err != nil {
		return "", fmt.Errorf("failed to write audit file: %w", err)
	}

	return filename, nil
}

// ensureAuditDir creates the audit directory if it doesn't exist
func (a *Auditor) ensureAuditDir() error {
	if _, err := os.Stat(a.AuditDir); os.IsNotExist(err) {
		if err := os.MkdirAll(a.AuditDir, 0755); err != nil {
			return fmt.Errorf("failed to create audit directory: %w", err)
		}
	}
	return nil
}
