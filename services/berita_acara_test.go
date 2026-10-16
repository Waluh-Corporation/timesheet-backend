package services

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"timesheet-backend/models"
)

func TestGenerateBeritaAcara_Validation(t *testing.T) {
	t.Run("nil user", func(t *testing.T) {
		_, err := GenerateBeritaAcara(BeritaAcaraInput{
			User:    nil,
			Witness: &models.User{Name: "Witness"},
		})
		if err == nil {
			t.Fatal("expected error for nil user, got nil")
		}
	})

	t.Run("nil witness", func(t *testing.T) {
		_, err := GenerateBeritaAcara(BeritaAcaraInput{
			User:    &models.User{Name: "User"},
			Witness: nil,
		})
		if err == nil {
			t.Fatal("expected error for nil witness, got nil")
		}
	})
}

func TestGenerateBeritaAcara_Success(t *testing.T) {
	user := &models.User{
		ID:         1,
		Name:       "Ahmad Faisal",
		Username:   "afaisal",
		BniID:      "987654",
		Department: "Digital Architecture",
		GroupName:  "Backend Core",
	}
	witness := &models.User{
		ID:       2,
		Name:     "Budi Saksi",
		Username: "bsaksi",
	}

	activities := []models.DailyActivity{
		{
			Date:      time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
			StartTime: "08:00",
			EndTime:   "17:00",
			Activity:  "Implement API Authentication",
		},
		{
			Date:      time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC),
			StartTime: "08:30",
			EndTime:   "17:30",
			Activity:  "Database Schema Optimization",
		},
	}

	approvers := []models.Approver{
		{
			Name:     "Citra TeamLeader",
			RoleType: models.ApproverRoleTeamLeader,
			IsActive: true,
		},
	}

	out, err := GenerateBeritaAcara(BeritaAcaraInput{
		User:       user,
		Witness:    witness,
		Month:      10,
		Year:       2026,
		Activities: activities,
		Approvers:  approvers,
	})
	if err != nil {
		t.Fatalf("GenerateBeritaAcara failed: %v", err)
	}

	if len(out) == 0 {
		t.Fatal("expected non-empty output")
	}

	// Read output zip to inspect contents
	zr, err := zip.NewReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("failed to read generated docx: %v", err)
	}

	files := make(map[string]string)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("failed to open %s: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("failed to read %s: %v", f.Name, err)
		}
		files[f.Name] = string(data)
	}

	docXML, ok := files["word/document.xml"]
	if !ok {
		t.Fatal("word/document.xml missing from output docx")
	}

	// Verify metadata replacement
	if !strings.Contains(docXML, "Ahmad Faisal") {
		t.Errorf("expected user name Ahmad Faisal in document.xml")
	}
	if !strings.Contains(docXML, "987654") {
		t.Errorf("expected BniID 987654 in document.xml")
	}
	if !strings.Contains(docXML, "Digital Architecture") {
		t.Errorf("expected Department Digital Architecture in document.xml")
	}
	if !strings.Contains(docXML, "Backend Core") {
		t.Errorf("expected GroupName Backend Core in document.xml")
	}

	// Verify activities
	if !strings.Contains(docXML, "01/10/2026") || !strings.Contains(docXML, "Kamis") {
		t.Errorf("expected activity 1 in document.xml")
	}
	if !strings.Contains(docXML, "02/10/2026") || !strings.Contains(docXML, "Jumat") {
		t.Errorf("expected activity 2 in document.xml")
	}
	if !strings.Contains(docXML, "Implement API Authentication") {
		t.Errorf("expected activity text in document.xml")
	}

	// Verify signatures names
	if !strings.Contains(docXML, "Budi Saksi") {
		t.Errorf("expected witness name Budi Saksi in document.xml")
	}
	if !strings.Contains(docXML, "Citra TeamLeader") {
		t.Errorf("expected team lead Citra TeamLeader in document.xml")
	}
}
