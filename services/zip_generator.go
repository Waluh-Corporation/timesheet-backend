package services

import (
	"archive/zip"
	"bytes"
	"fmt"
)

// ArchiveFile encapsulates a file name and its binary payload for ZIP packaging.
type ArchiveFile struct {
	Name string
	Data []byte
}

// CreateZipArchive bundles multiple files into an in-memory ZIP archive using archive/zip.
func CreateZipArchive(files ...ArchiveFile) ([]byte, error) {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	for _, file := range files {
		w, err := zw.Create(file.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to create zip entry %s: %w", file.Name, err)
		}
		if _, err := w.Write(file.Data); err != nil {
			return nil, fmt.Errorf("failed to write zip entry %s: %w", file.Name, err)
		}
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize zip archive: %w", err)
	}

	return buf.Bytes(), nil
}
