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
func ConvertExcelToPDF(excelBytes []byte, filename string) ([]byte, error) {
	// Create a secure temporary directory
	tmpDir, err := os.MkdirTemp("", "pdfconvert-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory for PDF conversion: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write Excel bytes to temporary file
	inputPath := filepath.Join(tmpDir, filename)
	if err := os.WriteFile(inputPath, excelBytes, 0644); err != nil {
		return nil, fmt.Errorf("failed to write excel bytes to temp file: %w", err)
	}

	// Run LibreOffice headless conversion
	// Command: libreoffice --headless --convert-to pdf --outdir <tmpDir> <inputPath>
	cmd := exec.Command("libreoffice", "--headless", "--convert-to", "pdf", "--outdir", tmpDir, inputPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("libreoffice headless conversion failed: %w, stderr: %s", err, stderr.String())
	}

	// Locate the generated PDF
	baseName := strings.TrimSuffix(filename, filepath.Ext(filename))
	outputPath := filepath.Join(tmpDir, baseName+".pdf")

	pdfBytes, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read converted PDF output: %w", err)
	}

	return pdfBytes, nil
}
