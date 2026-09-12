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

	setMeta := func(cellLabel, label, cellVal, val string) {
		_ = f.SetCellValue(sheet, cellLabel, label)
		_ = f.SetCellStyle(sheet, cellLabel, cellLabel, st.MetaLabelStyle)
		_ = f.SetCellValue(sheet, cellVal, val)
		_ = f.SetCellStyle(sheet, cellVal, cellVal, st.MetaValueStyle)
	}

	empID := in.User.EmployeeID
	if empID == "" {
		empID = in.User.BniID
	}
	dept := in.User.Department
	if dept == "" {
		dept = "WCH / WDL"
	}
	grp := in.User.GroupName
	if grp == "" {
		grp = "Junior Programmer"
	}

	setMeta("D2", "NAMA", "F2", ": "+in.User.Name)
	setMeta("D3", "NPP BNI", "F3", ": "+empID)
	setMeta("D4", "DIVISI", "F4", ": "+in.User.Division)
	setMeta("D5", "DEPARTEMEN", "F5", ": "+dept)
	setMeta("D6", "KELOMPOK", "F6", ": "+grp)
	setMeta("D7", "PERIODE ", "F7", fmt.Sprintf(": %s %d", indonesianMonth(in.Month), in.Year))

	legends := map[string]string{
		"F9": "H : Hadir", "G9": "C = Cuti", "H9": "I=Izin", "I9": "S = Sakit", "J9": "L = Lembur",
	}
	for cell, leg := range legends {
		_ = f.SetCellValue(sheet, cell, leg)
		_ = f.SetCellStyle(sheet, cell, cell, st.BoldLeftStyle)
	}
}

func writeSDDHeaders(f *excelize.File, sheet string, st *BuilderStyles) {
	type colHeader struct {
		from, to string
		val      string
	}
	mergedHeaders := []colHeader{
		{"A10", "A11", "No"},
		{"B10", "B11", "Tanggal"},
		{"C10", "C11", "Jam Masuk"},
		{"D10", "D11", "Jam Pulang"},
		{"E10", "E11", "Total Jam Kerja"},
		{"F10", "J10", "Status Kehadiran"},
		{"K10", "K11", "Project"},
		{"L10", "L11", "Project Code"},
		{"M10", "M11", "AIP Fitur"},
		{"N10", "N11", "Kegiatan"},
	}
	for _, mh := range mergedHeaders {
		if mh.from != mh.to {
			_ = f.MergeCell(sheet, mh.from, mh.to)
		}
		_ = f.SetCellValue(sheet, mh.from, mh.val)
		_ = f.SetCellStyle(sheet, mh.from, mh.to, st.HeaderStyle)
	}

	subHeaders := map[string]string{
		"F11": "Hadir", "G11": "Cuti", "H11": "Izin", "I11": "Sakit", "J11": "Lembur",
	}
	for cell, val := range subHeaders {
		_ = f.SetCellValue(sheet, cell, val)
		_ = f.SetCellStyle(sheet, cell, cell, st.HeaderGreyStyle)
	}
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

func writeSDDHolidayRow(f *excelize.File, sheet, rs string, row int, holiday string) {
	if holiday != "" {
		_ = f.SetCellValue(sheet, "N"+rs, holiday)
	} else {
		_ = f.SetCellValue(sheet, "N"+rs, "Weekend")
	}
	_ = f.SetRowHeight(sheet, row, 15)
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

		date := time.Date(in.Year, time.Month(in.Month), day, 0, 0, 0, 0, time.UTC)
		isWeekend := date.Weekday() == time.Saturday || date.Weekday() == time.Sunday
		holiday := in.Holidays[day]
		act, hasAct := byDay[day]

		isHolidayOrWeekend := isWeekend || holiday != ""

		centerStyle := st.DataCenterStyle
		centerWrapStyle := st.DataCenterWrapStyle
		dateStyle := st.DateStyle
		timeStyle := st.TimeStyle
		if isHolidayOrWeekend {
			centerStyle = st.GreyCenterStyle
			centerWrapStyle = st.GreyCenterWrapStyle
			dateStyle = st.GreyDateStyle
			timeStyle = st.GreyTimeStyle
		}

		for _, col := range allCols {
			_ = f.SetCellStyle(sheet, col+rs, col+rs, centerStyle)
		}
		_ = f.SetCellStyle(sheet, "N"+rs, "N"+rs, centerWrapStyle)

		_ = f.SetCellValue(sheet, "A"+rs, day)
		_ = f.SetCellValue(sheet, "B"+rs, date)
		_ = f.SetCellStyle(sheet, "B"+rs, "B"+rs, dateStyle)

		if hasAct {
			writeSDDActRow(f, sheet, rs, row, act, timeStyle)
		} else if isHolidayOrWeekend {
			writeSDDHolidayRow(f, sheet, rs, row, holiday)
		}
	}
}

func writeSDDSummaryAndSignatures(f *excelize.File, sheet string, in GenerationInput, st *BuilderStyles) {
	const sumRow = "43"
	formulas := map[string]string{
		"F": `COUNTIF(F12:F42,"v")`,
		"G": `COUNTIF(G12:G42,"v")`,
		"H": `COUNTIF(H12:H42,"v")`,
		"I": `COUNTIF(I12:I42,"v")`,
		"J": `COUNTIF(J12:J42,"v")`,
	}
	for col, formula := range formulas {
		cell := col + sumRow
		_ = f.SetCellFormula(sheet, cell, formula)
		_ = f.SetCellStyle(sheet, cell, cell, st.BoldCenterStyle)
	}

	WriteSignaturesLayout(f, sheet, 46, 5, []SignatureParty{
		{StartCol: "C", EndCol: "G", Title: "Pemohon", Name: in.User.Name, DatePrefix: sddNamePrefix},
		{StartCol: "H", EndCol: "K", Title: "Diperiksa,", Name: "", DatePrefix: sddNamePrefix},
		{StartCol: "L", EndCol: "M", Title: "Disetujui,", Name: "", DatePrefix: sddNamePrefix},
	}, st)
}

// buildSDDWorkbook generates the SDD timesheet Excel document purely from code.
func buildSDDWorkbook(in GenerationInput) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	sheet := indonesianMonth(in.Month)
	_ = f.SetSheetName(f.GetSheetName(0), sheet)

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
