package services

import (
	"bytes"
	"testing"
)

func TestPDFConstants(t *testing.T) {
	if !bytes.Equal(PDFMagicHeader, []byte("%PDF-")) {
		t.Fatalf("expected PDFMagicHeader to be %%PDF-, got %s", string(PDFMagicHeader))
	}
	if ErrPDFGeneration == nil {
		t.Fatal("expected ErrPDFGeneration to be non-nil")
	}
}
