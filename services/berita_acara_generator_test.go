package services_test

import (
	"archive/zip"
	"bytes"
	"testing"
	"time"

	"timesheet-backend/models"
	"timesheet-backend/services"
)

func TestGenerateBeritaAcaraDocx(t *testing.T) {
	user := &models.User{
		Name:       "Budi Santoso",
		BniID:      "12345",
		Department: "Wholesale Channel and Service Delivery",
		GroupName:  "Backend Engineer",
		Division:   "Wholesale Digital Delivery",
	}

	date1, _ := time.Parse("2006-01-02", "2026-09-01")
	date2, _ := time.Parse("2006-01-02", "2026-09-02")

	approverTL := models.Approver{
		Name:     "Team Lead Alpha",
		RoleType: models.ApproverRoleTeamLeader,
		IsActive: true,
	}
	approverDH := models.Approver{
		Name:     "Dept Head Beta",
		RoleType: models.ApproverRoleDepartmentHead,
		IsActive: true,
	}

	beritaAcaras := []models.BeritaAcara{
		{
			Date:       date1,
			Day:        "Selasa",
			StartTime:  "08:00",
			EndTime:    "17:00",
			Keterangan: "Lupa tap absensi masuk",
			TeamLeader: &approverTL,
		},
		{
			Date:       date2,
			Day:        "Rabu",
			StartTime:  "08:15",
			EndTime:    "17:30",
			Keterangan: "Lupa tap absensi pulang",
		},
	}

	companies := []string{"mii", "sdd", "adidata", "ntt", "unknown"}

	for _, comp := range companies {
		t.Run("generate for company "+comp, func(t *testing.T) {
			input := services.BeritaAcaraInput{
				CompanyCode:  comp,
				User:         user,
				Month:        9,
				Year:         2026,
				BeritaAcaras: beritaAcaras,
				Approvers:    []models.Approver{approverTL, approverDH},
			}

			docxBytes, err := services.GenerateBeritaAcaraDocx(input)
			if err != nil {
				t.Fatalf("GenerateBeritaAcaraDocx failed for %s: %v", comp, err)
			}
			if len(docxBytes) == 0 {
				t.Fatalf("expected non-empty docx bytes for %s", comp)
			}

			// Validate that the output is a valid zip (docx is a zip archive)
			zr, err := zip.NewReader(bytes.NewReader(docxBytes), int64(len(docxBytes)))
			if err != nil {
				t.Fatalf("generated docx is not a valid zip archive: %v", err)
			}

			hasDocXML := false
			for _, f := range zr.File {
				if f.Name == "word/document.xml" {
					hasDocXML = true
					break
				}
			}
			if !hasDocXML {
				t.Errorf("expected word/document.xml inside generated docx for %s", comp)
			}
		})
	}
}

func TestGenerateBeritaAcaraDocx_EmptyEntries(t *testing.T) {
	user := &models.User{
		Name:  "Test User",
		BniID: "99999",
	}

	input := services.BeritaAcaraInput{
		CompanyCode:  "mii",
		User:         user,
		Month:        9,
		Year:         2026,
		BeritaAcaras: nil,
	}

	docxBytes, err := services.GenerateBeritaAcaraDocx(input)
	if err != nil {
		t.Fatalf("failed with empty entries: %v", err)
	}
	if len(docxBytes) == 0 {
		t.Fatal("expected non-empty docx bytes")
	}
}
