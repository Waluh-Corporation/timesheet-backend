package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"timesheet-backend/auth"
	"timesheet-backend/dto/response"
	"timesheet-backend/models"
)

func TestBeritaAcaraHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cfg := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	authSvc := auth.NewService("test_secret", 24*time.Hour)
	srv, err := NewServer(tx, cfg, authSvc, nil, nil)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// 1. Find existing MII company
	var comp models.Company
	if err := tx.Where("code = ?", "MII").First(&comp).Error; err != nil {
		comp = models.Company{
			Code:     "MII",
			Name:     "PT Mitra Integrasi Informatika",
			IsActive: true,
		}
		_ = tx.Create(&comp).Error
	}

	user := models.User{
		Username:     "ba_handler_user",
		Email:        "ba_handler@example.com",
		Name:         "BA Handler User",
		Role:         models.RoleUser,
		PasswordHash: "hashed",
		IsActive:     true,
		CompanyID:    &comp.ID,
		Company:      "MII",
	}
	if err := tx.Create(&user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	user2 := models.User{
		Username:     "ba_handler_user2",
		Email:        "ba_handler2@example.com",
		Name:         "BA Handler User 2",
		Role:         models.RoleUser,
		PasswordHash: "hashed",
		IsActive:     true,
		CompanyID:    &comp.ID,
		Company:      "MII",
	}
	_ = tx.Create(&user2).Error

	t.Run("UpsertBeritaAcara validation and insert/update flow", func(t *testing.T) {
		// Invalid JSON
		wBad := httptest.NewRecorder()
		cBad, _ := gin.CreateTestContext(wBad)
		cBad.Request = httptest.NewRequest(http.MethodPost, "/api/v1/berita-acara", bytes.NewReader([]byte("invalid-json")))
		cBad.Set("userID", user.ID)
		srv.UpsertBeritaAcara(cBad)
		if wBad.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", wBad.Code)
		}

		// Invalid date
		wDate := httptest.NewRecorder()
		cDate, _ := gin.CreateTestContext(wDate)
		cDate.Request = httptest.NewRequest(http.MethodPost, "/api/v1/berita-acara", bytes.NewReader([]byte(`{"date":"not-a-date","keterangan":"Lupa tap"}`)))
		cDate.Set("userID", user.ID)
		srv.UpsertBeritaAcara(cDate)
		if wDate.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for bad date, got %d", wDate.Code)
		}

		// Valid Insert
		wOk := httptest.NewRecorder()
		cOk, _ := gin.CreateTestContext(wOk)
		body := `{"date":"2026-09-08","start_time":"08:00","end_time":"17:00","keterangan":"Lupa tap masuk"}`
		cOk.Request = httptest.NewRequest(http.MethodPost, "/api/v1/berita-acara", bytes.NewReader([]byte(body)))
		cOk.Set("userID", user.ID)
		srv.UpsertBeritaAcara(cOk)
		if wOk.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (body: %s)", wOk.Code, wOk.Body.String())
		}

		// Valid Update (upsert same date)
		wUp := httptest.NewRecorder()
		cUp, _ := gin.CreateTestContext(wUp)
		bodyUp := `{"date":"2026-09-08","start_time":"08:15","end_time":"17:15","keterangan":"Lupa tap masuk dan pulang"}`
		cUp.Request = httptest.NewRequest(http.MethodPost, "/api/v1/berita-acara", bytes.NewReader([]byte(bodyUp)))
		cUp.Set("userID", user.ID)
		srv.UpsertBeritaAcara(cUp)
		if wUp.Code != http.StatusOK {
			t.Fatalf("expected 200 for update, got %d", wUp.Code)
		}

		var count int64
		tx.Model(&models.BeritaAcara{}).Where("user_id = ? AND date = ?", user.ID, "2026-09-08").Count(&count)
		if count != 1 {
			t.Errorf("expected 1 record after upsert, got %d", count)
		}
	})

	t.Run("GetBeritaAcara detail, forbidden, and not found", func(t *testing.T) {
		// Find the created record
		var record models.BeritaAcara
		tx.Where("user_id = ?", user.ID).First(&record)
		if record.ID == 0 {
			t.Fatal("expected record to exist")
		}

		// Invalid ID
		wBadID := httptest.NewRecorder()
		cBadID, _ := gin.CreateTestContext(wBadID)
		cBadID.Params = gin.Params{{Key: "id", Value: "invalid"}}
		cBadID.Set("userID", user.ID)
		srv.GetBeritaAcara(cBadID)
		if wBadID.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", wBadID.Code)
		}

		// Not Found
		wNotFound := httptest.NewRecorder()
		cNotFound, _ := gin.CreateTestContext(wNotFound)
		cNotFound.Params = gin.Params{{Key: "id", Value: "999999"}}
		cNotFound.Set("userID", user.ID)
		srv.GetBeritaAcara(cNotFound)
		if wNotFound.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", wNotFound.Code)
		}

		// Forbidden: user2 tries to access user's record
		wForbid := httptest.NewRecorder()
		cForbid, _ := gin.CreateTestContext(wForbid)
		cForbid.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", record.ID)}}
		cForbid.Set("userID", user2.ID)
		srv.GetBeritaAcara(cForbid)
		if wForbid.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", wForbid.Code)
		}

		// Success: owner accesses record
		wOk := httptest.NewRecorder()
		cOk, _ := gin.CreateTestContext(wOk)
		cOk.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", record.ID)}}
		cOk.Set("userID", user.ID)
		srv.GetBeritaAcara(cOk)
		if wOk.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", wOk.Code)
		}

		var resp struct {
			Data response.BeritaAcaraDetailResponse `json:"data"`
		}
		_ = json.Unmarshal(wOk.Body.Bytes(), &resp)
		if resp.Data.Keterangan != "Lupa tap masuk dan pulang" {
			t.Errorf("expected updated keterangan, got %s", resp.Data.Keterangan)
		}
	})

	t.Run("ListBeritaAcara", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/berita-acara?month=9&year=2026", nil)
		c.Set("userID", user.ID)
		srv.ListBeritaAcara(c)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var paginatedResp response.PaginatedResponse
		_ = json.Unmarshal(w.Body.Bytes(), &paginatedResp)
		if paginatedResp.Pagination.TotalRows < 1 {
			t.Errorf("expected at least 1 total row, got %d", paginatedResp.Pagination.TotalRows)
		}
	})

	t.Run("GenerateTimesheet with type options", func(t *testing.T) {
		// Invalid type
		wInvalid := httptest.NewRecorder()
		cInvalid, _ := gin.CreateTestContext(wInvalid)
		cInvalid.Request = httptest.NewRequest(http.MethodPost, "/api/v1/timesheet/generate", bytes.NewReader([]byte(`{"month":9,"year":2026,"type":"invalid_type"}`)))
		cInvalid.Set("userID", user.ID)
		srv.GenerateTimesheet(cInvalid)
		if cInvalid.Writer.Status() != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid type, got %d", cInvalid.Writer.Status())
		}

		// type: "timesheet"
		wTimesheet := httptest.NewRecorder()
		cTimesheet, _ := gin.CreateTestContext(wTimesheet)
		cTimesheet.Request = httptest.NewRequest(http.MethodPost, "/api/v1/timesheet/generate", bytes.NewReader([]byte(`{"month":9,"year":2026,"type":"timesheet"}`)))
		cTimesheet.Set("userID", user.ID)
		srv.GenerateTimesheet(cTimesheet)
		if wTimesheet.Code != http.StatusOK {
			t.Fatalf("expected 200 for timesheet, got %d", wTimesheet.Code)
		}
		if !strings.Contains(wTimesheet.Header().Get("Content-Disposition"), ".xlsx") {
			t.Errorf("expected .xlsx attachment header, got %s", wTimesheet.Header().Get("Content-Disposition"))
		}

		// type: "berita_acara"
		wBA := httptest.NewRecorder()
		cBA, _ := gin.CreateTestContext(wBA)
		cBA.Request = httptest.NewRequest(http.MethodPost, "/api/v1/timesheet/generate", bytes.NewReader([]byte(`{"month":9,"year":2026,"type":"berita_acara"}`)))
		cBA.Set("userID", user.ID)
		srv.GenerateTimesheet(cBA)
		if wBA.Code != http.StatusOK {
			t.Fatalf("expected 200 for berita_acara, got %d (body: %s)", wBA.Code, wBA.Body.String())
		}
		if !strings.Contains(wBA.Header().Get("Content-Disposition"), ".docx") {
			t.Errorf("expected .docx attachment header, got %s", wBA.Header().Get("Content-Disposition"))
		}

		// type: "both"
		wBoth := httptest.NewRecorder()
		cBoth, _ := gin.CreateTestContext(wBoth)
		cBoth.Request = httptest.NewRequest(http.MethodPost, "/api/v1/timesheet/generate", bytes.NewReader([]byte(`{"month":9,"year":2026,"type":"both"}`)))
		cBoth.Set("userID", user.ID)
		srv.GenerateTimesheet(cBoth)
		if wBoth.Code != http.StatusOK {
			t.Fatalf("expected 200 for both, got %d (body: %s)", wBoth.Code, wBoth.Body.String())
		}
		if !strings.Contains(wBoth.Header().Get("Content-Disposition"), ".zip") {
			t.Errorf("expected .zip attachment header, got %s", wBoth.Header().Get("Content-Disposition"))
		}

		// Validate that the ZIP body contains both xlsx and docx
		zipBytes := wBoth.Body.Bytes()
		zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
		if err != nil {
			t.Fatalf("failed to read generated zip: %v", err)
		}
		if len(zr.File) != 2 {
			t.Fatalf("expected 2 files in zip, got %d", len(zr.File))
		}
		hasXLSX := false
		hasDOCX := false
		for _, f := range zr.File {
			if strings.HasSuffix(f.Name, ".xlsx") {
				hasXLSX = true
			}
			if strings.HasSuffix(f.Name, ".docx") {
				hasDOCX = true
			}
		}
		if !hasXLSX || !hasDOCX {
			t.Errorf("zip must contain both .xlsx and .docx, got hasXLSX=%v, hasDOCX=%v", hasXLSX, hasDOCX)
		}
	})
}
