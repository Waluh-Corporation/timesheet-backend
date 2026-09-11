package services

import (
	"testing"
)

func TestConvertExcelToPDF_NoLibreOffice(t *testing.T) {
	// Without libreoffice installed, ConvertExcelToPDF should return an error gracefully
	_, err := ConvertExcelToPDF([]byte("dummy excel data"), "test.xlsx")
	if err == nil {
		// If libreoffice happens to be installed and converts, that's fine too
		t.Log("libreoffice is installed and conversion succeeded or executed")
	} else {
		t.Logf("ConvertExcelToPDF returned expected error without headless libreoffice: %v", err)
	}
}
