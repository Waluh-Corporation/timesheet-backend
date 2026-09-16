package services

import (
	"bytes"
	"fmt"

	"github.com/xuri/excelize/v2"

	"timesheet-backend/dto/response"
	"timesheet-backend/models"
)

// SummaryReportInput holds the data required to build the historical summary workbook.
type SummaryReportInput struct {
	User    *models.User
	Summary *response.TimesheetSummaryResponse
}

// MonthNameIndonesian returns the full Indonesian month name for month 1-12.
func MonthNameIndonesian(month int) string {
	names := []string{
		"Januari", "Februari", "Maret", "April", "Mei", "Juni",
		"Juli", "Agustus", "September", "Oktober", "November", "Desember",
	}
	if month >= 1 && month <= 12 {
		return names[month-1]
	}
	return fmt.Sprintf("Bulan %d", month)
}

// BuildSummaryWorkbook generates a formatted Excel workbook for monthly/yearly timesheet summary.
func BuildSummaryWorkbook(in SummaryReportInput) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	sheet := "Rekapitulasi"
	f.SetSheetName("Sheet1", sheet)

	st, err := NewBuilderStyles(f)
	if err != nil {
		return nil, err
	}

	// Set column widths
	colWidths := map[string]float64{
		"A": 6,  // No
		"B": 16, // Bulan
		"C": 18, // Hari Kerja Efektif
		"D": 14, // Hari Terisi
		"E": 12, // Hadir (P)
		"F": 12, // Sakit (S)
		"G": 12, // Izin (PM)
		"H": 12, // Cuti (V)
		"I": 12, // Dinas (BT)
		"J": 14, // Tidak Bekerja (X)
		"K": 20, // Total Jam Kerja
		"L": 20, // Total Jam Lembur
		"M": 20, // Status Kelengkapan
	}
	for col, w := range colWidths {
		_ = f.SetColWidth(sheet, col, col, w)
	}

	// Title Block
	title := fmt.Sprintf("LAPORAN REKAPITULASI TIMESHEET & AKTIVITAS TAHUNAN - %d", in.Summary.Year)
	if in.Summary.Month != nil {
		title = fmt.Sprintf("LAPORAN REKAPITULASI TIMESHEET & AKTIVITAS - %s %d", MonthNameIndonesian(*in.Summary.Month), in.Summary.Year)
	}
	_ = f.MergeCell(sheet, "A1", "M1")
	_ = f.SetCellValue(sheet, "A1", title)
	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14, Color: "#1F4E79", Family: "Calibri"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	_ = f.SetCellStyle(sheet, "A1", "M1", titleStyle)
	_ = f.SetRowHeight(sheet, 1, 30)

	// User Metadata Block
	userName := in.User.Name
	empID := ResolveEmployeeID(in.User)
	company := in.User.Company
	if in.User.CompanyRel != nil && in.User.CompanyRel.Name != "" {
		company = in.User.CompanyRel.Name
	}
	unit := in.User.Department
	if in.User.Division != "" {
		unit = in.User.Division + " / " + unit
	}

	WriteMetaField(f, sheet, "A3", "Nama Pegawai", "C3", ": "+userName, st)
	WriteMetaField(f, sheet, "A4", "NIP / ID", "C4", ": "+empID, st)
	WriteMetaField(f, sheet, "A5", "Perusahaan", "C5", ": "+company, st)
	WriteMetaField(f, sheet, "A6", "Unit / Divisi", "C6", ": "+unit, st)

	// Table Headers at Row 8
	headers := []HeaderColumn{
		{From: "A8", To: "A8", Val: "NO"},
		{From: "B8", To: "B8", Val: "BULAN"},
		{From: "C8", To: "C8", Val: "HARI KERJA EFEKTIF"},
		{From: "D8", To: "D8", Val: "HARI TERISI"},
		{From: "E8", To: "E8", Val: "HADIR (P)"},
		{From: "F8", To: "F8", Val: "SAKIT (S)"},
		{From: "G8", To: "G8", Val: "IZIN (PM)"},
		{From: "H8", To: "H8", Val: "CUTI (V)"},
		{From: "I8", To: "I8", Val: "DINAS (BT)"},
		{From: "J8", To: "J8", Val: "OFF / KELUAR (X)"},
		{From: "K8", To: "K8", Val: "TOTAL JAM KERJA"},
		{From: "L8", To: "L8", Val: "TOTAL JAM LEMBUR"},
		{From: "M8", To: "M8", Val: "STATUS KELENGKAPAN"},
	}
	WriteTableHeaders(f, sheet, headers, nil, st)
	_ = f.SetRowHeight(sheet, 8, 25)

	// Data Rows
	startRow := 9
	cols := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M"}
	for i, m := range in.Summary.Months {
		row := startRow + i
		rs := fmt.Sprintf("%d", row)

		for _, col := range cols {
			_ = f.SetCellStyle(sheet, col+rs, col+rs, st.DataCenterStyle)
		}

		_ = f.SetCellValue(sheet, "A"+rs, i+1)
		_ = f.SetCellValue(sheet, "B"+rs, m.MonthName)
		_ = f.SetCellValue(sheet, "C"+rs, m.WorkingDays)
		_ = f.SetCellValue(sheet, "D"+rs, m.DaysFilled)
		_ = f.SetCellValue(sheet, "E"+rs, m.AttendanceBreakdown["P"])
		_ = f.SetCellValue(sheet, "F"+rs, m.AttendanceBreakdown["S"])
		_ = f.SetCellValue(sheet, "G"+rs, m.AttendanceBreakdown["PM"])
		_ = f.SetCellValue(sheet, "H"+rs, m.AttendanceBreakdown["V"])
		_ = f.SetCellValue(sheet, "I"+rs, m.AttendanceBreakdown["BT"])
		_ = f.SetCellValue(sheet, "J"+rs, m.AttendanceBreakdown["X"])
		_ = f.SetCellValue(sheet, "K"+rs, fmt.Sprintf("%.1f", m.WorkingHours))
		_ = f.SetCellValue(sheet, "L"+rs, fmt.Sprintf("%.1f", m.OvertimeHours))

		statusText := "Kosong"
		if m.IsComplete {
			statusText = "Lengkap"
		} else if m.DaysFilled > 0 {
			statusText = "Belum Lengkap"
		}
		_ = f.SetCellValue(sheet, "M"+rs, statusText)
		_ = f.SetRowHeight(sheet, row, 20)
	}

	// Total Row
	lastDataRow := startRow + len(in.Summary.Months) - 1
	totalRow := lastDataRow + 1
	trs := fmt.Sprintf("%d", totalRow)

	_ = f.MergeCell(sheet, "A"+trs, "B"+trs)
	_ = f.SetCellValue(sheet, "A"+trs, "TOTAL")
	for _, col := range cols {
		_ = f.SetCellStyle(sheet, col+trs, col+trs, st.BoldCenterStyle)
	}

	if len(in.Summary.Months) > 0 {
		fdr := fmt.Sprintf("%d", startRow)
		ldr := fmt.Sprintf("%d", lastDataRow)
		_ = f.SetCellFormula(sheet, "C"+trs, fmt.Sprintf("SUM(C%s:C%s)", fdr, ldr))
		_ = f.SetCellFormula(sheet, "D"+trs, fmt.Sprintf("SUM(D%s:D%s)", fdr, ldr))
		_ = f.SetCellFormula(sheet, "E"+trs, fmt.Sprintf("SUM(E%s:E%s)", fdr, ldr))
		_ = f.SetCellFormula(sheet, "F"+trs, fmt.Sprintf("SUM(F%s:F%s)", fdr, ldr))
		_ = f.SetCellFormula(sheet, "G"+trs, fmt.Sprintf("SUM(G%s:G%s)", fdr, ldr))
		_ = f.SetCellFormula(sheet, "H"+trs, fmt.Sprintf("SUM(H%s:H%s)", fdr, ldr))
		_ = f.SetCellFormula(sheet, "I"+trs, fmt.Sprintf("SUM(I%s:I%s)", fdr, ldr))
		_ = f.SetCellFormula(sheet, "J"+trs, fmt.Sprintf("SUM(J%s:J%s)", fdr, ldr))
		_ = f.SetCellFormula(sheet, "K"+trs, fmt.Sprintf("SUM(K%s:K%s)", fdr, ldr))
		_ = f.SetCellFormula(sheet, "L"+trs, fmt.Sprintf("SUM(L%s:L%s)", fdr, ldr))
	}
	_ = f.SetRowHeight(sheet, totalRow, 22)

	SetWorkbookProperties(f, title, in.User.Name)

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
