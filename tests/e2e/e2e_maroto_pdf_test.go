package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"timesheet-backend/models"
	"timesheet-backend/services"
)

// =============================================================================
// TIER 1 & TIER 2: PURE-GO MAROTO V2 PDF ARCHITECTURE & CALCULATOR TESTS
// =============================================================================

// TestE2E_MarotoV2_TimesheetPresentationModel verifies Feature 10 (Canonical Presentation Model)
// and business rule calculations for working days, weekends, holidays, and overtime.
func TestE2E_MarotoV2_TimesheetPresentationModel(t *testing.T) {
	// Prepare test user
	user := &models.User{
		Name:       "Budi Santoso",
		BniID:      "12345",
		Department: "Information Technology",
		Division:   "Technology",
		Site:       "Jakarta",
	}

	// Prepare test data for September 2026 (30 days)
	// Sep 5 & 6 are Saturday & Sunday.
	// Sep 5 has overtime activity (08:30 - 17:30).
	// Sep 6 has NO activity.
	// Sep 16 has public holiday "Maulid Nabi".
	// Sep 16 has overtime activity.
	activities := []models.DailyActivity{
		{
			Date:      time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
			StartTime: "08:30",
			EndTime:   "17:30",
			Status:    "Present",
			Activity:  "Standard regular workday",
		},
		{
			// Saturday Overtime
			Date:      time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC),
			StartTime: "09:00",
			EndTime:   "15:00",
			Status:    "Present",
			Activity:  "Emergency production weekend deployment",
		},
		{
			// Public Holiday Overtime
			Date:      time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC),
			StartTime: "10:00",
			EndTime:   "16:00",
			Status:    "Present",
			Activity:  "Holiday hotline support",
		},
	}

	holidays := map[int]string{
		16: "Maulid Nabi",
	}

	input := services.GenerationInput{
		CompanyCode: "mii",
		User:        user,
		Year:        2026,
		Month:       9,
		Activities:  activities,
		Holidays:    holidays,
	}

	model, err := services.CalculateTimesheetModel(&input)
	require.NoError(t, err)
	require.NotNil(t, model)

	// Validate metadata
	assert.Equal(t, "Budi Santoso", model.EmployeeName)
	assert.Equal(t, "12345", model.EmployeeID)
	assert.Equal(t, 2026, model.Year)
	assert.Equal(t, 9, model.Month)

	// Validate calendar rows count (September has 30 days)
	require.Len(t, model.Rows, 30)

	// Day 1 (Regular Working Day)
	row1 := model.Rows[0]
	assert.Equal(t, 1, row1.Day)
	assert.False(t, row1.IsWeekend)
	assert.False(t, row1.IsHoliday)
	assert.True(t, row1.HasActivity)
	assert.Equal(t, "08:30", row1.StartTime)
	assert.Equal(t, "17:30", row1.EndTime)
	assert.Equal(t, "Standard regular workday", row1.Activity)

	// Day 5 (Saturday with Overtime Activity -> Feature R1 Dynamic Styling)
	row5 := model.Rows[4]
	assert.Equal(t, 5, row5.Day)
	assert.True(t, row5.IsWeekend)
	assert.True(t, row5.HasActivity)
	assert.True(t, row5.IsOvertime, "Weekend with activity must be marked as overtime")
	assert.Equal(t, "09:00", row5.StartTime)
	assert.Equal(t, "15:00", row5.EndTime)
	assert.Equal(t, "Emergency production weekend deployment", row5.Activity)

	// Day 6 (Sunday without Activity -> Feature R2 Default Fallback)
	row6 := model.Rows[5]
	assert.Equal(t, 6, row6.Day)
	assert.True(t, row6.IsWeekend)
	assert.False(t, row6.HasActivity)
	assert.False(t, row6.IsOvertime)
	assert.Empty(t, row6.StartTime)
	assert.Empty(t, row6.EndTime)
	assert.Equal(t, "Weekend", row6.Remark, "Inactive weekend must default to 'Weekend' remark")

	// Day 16 (Holiday with Overtime Activity)
	row16 := model.Rows[15]
	assert.Equal(t, 16, row16.Day)
	assert.True(t, row16.IsHoliday)
	assert.True(t, row16.HasActivity)
	assert.True(t, row16.IsOvertime)
	assert.Equal(t, "Maulid Nabi", row16.HolidayDesc)
	assert.Equal(t, "10:00", row16.StartTime)

	// Validate summary calculations
	assert.Greater(t, model.Summary.TotalWorkingHours, 0.0)
	assert.Greater(t, model.Summary.TotalOvertimeHours, 0.0)
}

// TestE2E_SchemaDrivenLayoutEngine verifies Feature 11 (12-grid proportional layouts).
func TestE2E_SchemaDrivenLayoutEngine(t *testing.T) {
	// Maroto v2 requires that the sum of column grid widths in a row strictly equals 12.
	timesheetCols := services.StandardTimesheetPDFColumns()
	require.NotEmpty(t, timesheetCols)

	tsGridSum := 0
	for _, col := range timesheetCols {
		tsGridSum += col.GridRatio
		assert.NotEmpty(t, col.Key)
		assert.NotEmpty(t, col.Title)
		assert.Greater(t, col.GridRatio, 0)
	}
	assert.Equal(t, 12, tsGridSum, "Timesheet PDF columns must sum to exactly 12 grid units")

	baCols := services.BeritaAcaraPDFColumns()
	require.NotEmpty(t, baCols)

	baGridSum := 0
	for _, col := range baCols {
		baGridSum += col.GridRatio
		assert.NotEmpty(t, col.Key)
		assert.NotEmpty(t, col.Title)
		assert.Greater(t, col.GridRatio, 0)
	}
	assert.Equal(t, 12, baGridSum, "Berita Acara PDF columns must sum to exactly 12 grid units")
}

// TestE2E_BuildTimesheetPDF_PureGo verifies Feature 12 (Pure-Go Timesheet PDF).
func TestE2E_BuildTimesheetPDF_PureGo(t *testing.T) {
	ctx := context.Background()

	input := services.GenerationInput{
		CompanyCode: "mii",
		Year:        2026,
		Month:       9,
		User: &models.User{
			Name:       "Budi Santoso",
			BniID:      "12345",
			Department: "Engineering",
		},
		Activities: []models.DailyActivity{
			{
				Date:      time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
				StartTime: "08:30",
				EndTime:   "17:30",
				Status:    "Present",
				Activity:  "Building high-performance PDF engine",
			},
		},
		Holidays: map[int]string{},
	}

	model, err := services.CalculateTimesheetModel(&input)
	require.NoError(t, err)

	pdfBytes, err := services.BuildTimesheetPDF(ctx, model)
	require.NoError(t, err)
	require.NotEmpty(t, pdfBytes)

	// Verification Oracle:
	// 1. Begins with %PDF- header magic bytes
	assert.True(t, strings.HasPrefix(string(pdfBytes[:8]), "%PDF-"), "Output must start with %PDF- header")
	// 2. Minimum length check (> 1KB)
	assert.Greater(t, len(pdfBytes), 1024, "Generated PDF must exceed 1024 bytes")
	// 3. Contains PDF EOF marker
	assert.Contains(t, string(pdfBytes[len(pdfBytes)-100:]), "%%EOF", "Output must terminate with %%EOF marker")
}

// TestE2E_BuildBeritaAcaraPDF_PureGo verifies Feature 13 (Pure-Go Berita Acara PDF)
// with dual signatures and QR code verification block.
func TestE2E_BuildBeritaAcaraPDF_PureGo(t *testing.T) {
	ctx := context.Background()

	rawPNG, _ := services.GenerateQRCodePNG("https://timesheet.local/verify/doc/token-test-123", 100)

	payload := services.BeritaAcaraPayload{
		EmployeeName:           "Budi Santoso",
		NPP:                    "12345",
		Department:             "Information Technology",
		GroupName:              "Enterprise Architecture",
		WitnessName:            "Dewi Sartika",
		TeamLeadName:           "Ir. Soekarno",
		EmployeeSignatureBytes: rawPNG,
		WitnessSignatureBytes:  rawPNG,
		QRCodeBytes:            rawPNG,
		IsSigned:               true,
		Items: []services.BeritaAcaraItem{
			{
				Date:      time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC),
				Day:       "Sabtu",
				StartTime: "08:30",
				EndTime:   "17:30",
				Notes:     "Tapping badge failure at entrance gate",
			},
			{
				Date:      time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC),
				Day:       "Sabtu",
				StartTime: "09:00",
				EndTime:   "18:00",
				Notes:     "Biometric scanner timeout",
			},
		},
	}

	pdfBytes, err := services.BuildBeritaAcaraPDF(ctx, &payload)
	require.NoError(t, err)
	require.NotEmpty(t, pdfBytes)

	// Verification Oracle:
	assert.True(t, strings.HasPrefix(string(pdfBytes[:8]), "%PDF-"))
	assert.Greater(t, len(pdfBytes), 1024)
	assert.Contains(t, string(pdfBytes[len(pdfBytes)-100:]), "%%EOF")
}

// TestE2E_BuildBeritaAcaraPDF_EmptyIncidentList verifies Boundary BND-10 (Empty Incidents).
func TestE2E_BuildBeritaAcaraPDF_EmptyIncidentList(t *testing.T) {
	ctx := context.Background()

	payload := services.BeritaAcaraPayload{
		EmployeeName: "Budi Santoso",
		NPP:          "12345",
		Department:   "Engineering",
		WitnessName:  "Dewi Sartika",
		Items:        []services.BeritaAcaraItem{}, // Empty incidents list
		IsSigned:     false,
	}

	// Must render cleanly without panic or runtime error
	pdfBytes, err := services.BuildBeritaAcaraPDF(ctx, &payload)
	require.NoError(t, err)
	require.NotEmpty(t, pdfBytes)
	assert.True(t, strings.HasPrefix(string(pdfBytes[:8]), "%PDF-"))
}
