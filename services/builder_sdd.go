package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"timesheet-backend/assets"
	"timesheet-backend/models"
)

// indonesianMonth returns Indonesian month names for SDD.
func indonesianMonth(m int) string {
	months := []string{
		"Januari", "Februari", "Maret", "April", "Mei", "Juni",
		"Juli", "Agustus", "September", "Oktober", "November", "Desember",
	}
	if m >= 1 && m <= 12 {
		return months[m-1]
	}
	return time.Month(m).String()
}

const sddNamePrefix = "Nama : "

func setSDDColWidths(f *excelize.File, sheet string) {
	widths := map[string]float64{
		"A": 6, "B": 13, "C": 12, "D": 12, "E": 14,
		"F": 8, "G": 8, "H": 8, "I": 8, "J": 8,
		"K": 20, "L": 15, "M": 15, "N": 40,
	}
	for col, w := range widths {
		_ = f.SetColWidth(sheet, col, col, w)
	}
}

func writeSDDMetadata(f *excelize.File, sheet string, in GenerationInput, st *BuilderStyles) {
	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 14, Family: "Calibri"},
	})
	_ = f.SetCellValue(sheet, "B1", "ABSENSI MANUAL")
	_ = f.SetCellStyle(sheet, "B1", "B1", titleStyle)

	empID := ResolveEmployeeID(in.User)
	dept := in.User.Department
	if dept == "" {
		dept = "WCH / WDL"
	}
	grp := in.User.GroupName
	if grp == "" {
		grp = "Junior Programmer"
	}

	WriteMetaField(f, sheet, "D2", "NAMA", "F2", ": "+in.User.Name, st)
	WriteMetaField(f, sheet, "D3", "NPP BNI", "F3", ": "+empID, st)
	WriteMetaField(f, sheet, "D4", "DIVISI", "F4", ": "+in.User.Division, st)
	WriteMetaField(f, sheet, "D5", "DEPARTEMEN", "F5", ": "+dept, st)
	WriteMetaField(f, sheet, "D6", "KELOMPOK", "F6", ": "+grp, st)
	WriteMetaField(f, sheet, "D7", "PERIODE ", "F7", fmt.Sprintf(": %s %d", indonesianMonth(in.Month), in.Year), st)

	legends := map[string]string{
		"F9": "H : Hadir", "G9": "C = Cuti", "H9": "I=Izin", "I9": "S = Sakit", "J9": "L = Lembur",
	}
	for cell, leg := range legends {
		_ = f.SetCellValue(sheet, cell, leg)
		_ = f.SetCellStyle(sheet, cell, cell, st.BoldLeftStyle)
	}
}

func writeSDDHeaders(f *excelize.File, sheet string, st *BuilderStyles) {
	headers := []HeaderColumn{
		{From: "A10", To: "A11", Val: "No"},
		{From: "B10", To: "B11", Val: "Tanggal"},
		{From: "C10", To: "C11", Val: "Jam Masuk"},
		{From: "D10", To: "D11", Val: "Jam Pulang"},
		{From: "E10", To: "E11", Val: "Total Jam Kerja"},
		{From: "F10", To: "J10", Val: "Status Kehadiran"},
		{From: "K10", To: "K11", Val: "Project"},
		{From: "L10", To: "L11", Val: "Project Code"},
		{From: "M10", To: "M11", Val: "AIP Fitur"},
		{From: "N10", To: "N11", Val: "Kegiatan"},
	}
	subHeaders := map[string]string{
		"F11": "Hadir", "G11": "Cuti", "H11": "Izin", "I11": "Sakit", "J11": "Lembur",
	}
	WriteTableHeaders(f, sheet, headers, subHeaders, st)
}

func writeSDDActRow(f *excelize.File, sheet, rs string, row int, act models.DailyActivity, timeStyle int) {
	hasStart, hasEnd := WriteTimeCells(f, sheet, "C"+rs, "D"+rs, act.StartTime, act.EndTime, timeStyle)
	if hasStart && hasEnd {
		_ = f.SetCellFormula(sheet, "E"+rs, fmt.Sprintf("D%s-C%s", rs, rs))
		_ = f.SetCellStyle(sheet, "E"+rs, "E"+rs, timeStyle)
	}

	switch strings.ToUpper(strings.TrimSpace(act.Status)) {
	case "H", "P", "HADIR", "PRESENT":
		_ = f.SetCellValue(sheet, "F"+rs, "v")
	case "C", "CUTI", "V", "VACATION":
		_ = f.SetCellValue(sheet, "G"+rs, "v")
	case "I", "IZIN", "PM", "PERMIT":
		_ = f.SetCellValue(sheet, "H"+rs, "v")
	case "S", "SAKIT", "SICK":
		_ = f.SetCellValue(sheet, "I"+rs, "v")
	case "L", "LEMBUR":
		_ = f.SetCellValue(sheet, "J"+rs, "v")
	}

	_ = f.SetCellValue(sheet, "K"+rs, act.GetProjectName())
	_ = f.SetCellValue(sheet, "L"+rs, act.GetProjectCode())
	_ = f.SetCellValue(sheet, "N"+rs, act.Activity)

	h := calculateRowHeight(act.Activity, act.GetProjectName(), act.GetProjectCode(), "", "", "")
	_ = f.SetRowHeight(sheet, row, h)
}

func writeSDDDailyRows(f *excelize.File, sheet string, in GenerationInput, st *BuilderStyles) {
	byDay := BuildByDayMap(in.Activities)

	daysInMonth := GetDaysInMonth(in.Year, in.Month)
	const firstRow = 12
	allCols := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N"}

	for day := 1; day <= 31; day++ {
		row := firstRow + (day - 1)
		rs := fmt.Sprintf("%d", row)

		if day > daysInMonth {
			ApplyBlankPaddingRow(f, sheet, row, allCols, []string{"N"}, st)
			continue
		}

		dsc := ResolveDayStyleContext(in.Year, in.Month, day, in.Holidays, byDay, st)
		ApplyDayRowStyles(f, sheet, "B", rs, allCols, []string{"N"}, dsc)
		_ = f.SetCellValue(sheet, "A"+rs, day)

		if dsc.HasActivity {
			writeSDDActRow(f, sheet, rs, row, dsc.Activity, dsc.TimeStyle)
		} else if dsc.IsHolidayOrWeekend {
			WriteHolidayRemarkRow(f, sheet, "N", rs, row, dsc.Holiday)
		}
	}
}

func writeSDDSummaryAndSignatures(f *excelize.File, sheet string, in GenerationInput, st *BuilderStyles) {
	formulas := map[string]string{
		"F": `COUNTIF(F12:F42,"v")`,
		"G": `COUNTIF(G12:G42,"v")`,
		"H": `COUNTIF(H12:H42,"v")`,
		"I": `COUNTIF(I12:I42,"v")`,
		"J": `COUNTIF(J12:J42,"v")`,
	}
	WriteColumnFormulas(f, sheet, "43", formulas, st.BoldCenterStyle)

	tlName, dhName := ResolveApprovers(in)

	WriteSignaturesLayout(f, sheet, 46, 5, []SignatureParty{
		{StartCol: "C", EndCol: "G", Title: "Pemohon", Name: in.User.Name, DatePrefix: sddNamePrefix},
		{StartCol: "H", EndCol: "K", Title: "Diperiksa,", Name: tlName, DatePrefix: sddNamePrefix},
		{StartCol: "L", EndCol: "M", Title: "Disetujui,", Name: dhName, DatePrefix: sddNamePrefix},
	}, st)
}

// buildSDDWorkbook generates the SDD timesheet Excel document purely from code.
func buildSDDWorkbook(in GenerationInput) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	sheet := indonesianMonth(in.Month)
	_ = f.SetSheetName(f.GetSheetName(0), sheet)
	userName := ""
	if in.User != nil {
		userName = in.User.Name
	}
	SetWorkbookProperties(f, "Timesheet SDD", userName)

	st, err := NewBuilderStyles(f)
	if err != nil {
		return nil, fmt.Errorf("create builder styles: %w", err)
	}

	setSDDColWidths(f, sheet)

	if len(assets.SDDLogo) > 0 {
		_ = addHeaderLogo(f, sheet, "A2", assets.SDDLogo, ".jpg", 0.6, 0.6)
	}

	writeSDDMetadata(f, sheet, in, st)
	writeSDDHeaders(f, sheet, st)
	writeSDDDailyRows(f, sheet, in, st)
	writeSDDSummaryAndSignatures(f, sheet, in, st)

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
