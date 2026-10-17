package services

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ConvertExcelToPDF converts spreadsheet bytes to PDF locally using LibreOffice
func ConvertExcelToPDF(excelBytes []byte, _ string) ([]byte, error) {
	// Create a secure temporary directory
	tmpDir, err := os.MkdirTemp("", "pdfconvert-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory for PDF conversion: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Fixed internal filename eliminates CLI argument injection and traversal
	const inputFileName = "document.xlsx"
	inputPath := filepath.Join(tmpDir, inputFileName)
	if err := os.WriteFile(inputPath, excelBytes, 0600); err != nil {
		return nil, fmt.Errorf("failed to write excel bytes to temp file: %w", err)
	}

	binPath, err := exec.LookPath("libreoffice")
	if err != nil {
		binPath = "libreoffice"
	}

	// Run LibreOffice headless conversion
	//nolint:gosec // G204: binary is looked up and paths are within isolated temp dir
	cmd := exec.Command(binPath, "--headless", "--convert-to", "pdf", "--outdir", tmpDir, inputPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("libreoffice headless conversion failed: %w, stderr: %s", err, stderr.String())
	}

	// Locate the generated PDF deterministically and verify directory boundary
	outputPath := filepath.Join(tmpDir, "document.pdf")
	cleanOutput := filepath.Clean(outputPath)
	cleanTmpDir := filepath.Clean(tmpDir)
	if !strings.HasPrefix(cleanOutput, cleanTmpDir+string(filepath.Separator)) {
		return nil, fmt.Errorf("security violation: output path escaped temporary directory")
	}

	pdfBytes, err := os.ReadFile(cleanOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to read converted PDF output: %w", err)
	}

	return pdfBytes, nil
}
