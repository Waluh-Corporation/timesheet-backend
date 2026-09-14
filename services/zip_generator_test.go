package services_test

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"

	"timesheet-backend/services"
)

func TestCreateZipArchive(t *testing.T) {
	file1 := services.ArchiveFile{
		Name: "Timesheet_test.xlsx",
		Data: []byte("fake-excel-content"),
	}
	file2 := services.ArchiveFile{
		Name: "Berita_Acara_test.docx",
		Data: []byte("fake-word-content"),
	}

	zipBytes, err := services.CreateZipArchive(file1, file2)
	if err != nil {
		t.Fatalf("CreateZipArchive failed: %v", err)
	}
	if len(zipBytes) == 0 {
		t.Fatal("expected non-empty zip bytes")
	}

	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("invalid zip archive: %v", err)
	}

	if len(zr.File) != 2 {
		t.Fatalf("expected 2 files in zip, got %d", len(zr.File))
	}

	foundMap := make(map[string]string)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("failed to open file %s: %v", f.Name, err)
		}
		content, _ := io.ReadAll(rc)
		rc.Close()
		foundMap[f.Name] = string(content)
	}

	if foundMap["Timesheet_test.xlsx"] != "fake-excel-content" {
		t.Errorf("unexpected content for Timesheet_test.xlsx: %s", foundMap["Timesheet_test.xlsx"])
	}
	if foundMap["Berita_Acara_test.docx"] != "fake-word-content" {
		t.Errorf("unexpected content for Berita_Acara_test.docx: %s", foundMap["Berita_Acara_test.docx"])
	}
}
