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

// buildSDDWorkbook generates the SDD timesheet Excel document purely from code.
func buildSDDWorkbook(in GenerationInput) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	sheet := indonesianMonth(in.Month)
	_ = f.SetSheetName(f.GetSheetName(0), sheet)

	st, err := NewBuilderStyles(f)
	if err != nil {
		return nil, fmt.Errorf("create builder styles: %w", err)
	}

	// 1. Column Widths
	widths := map[string]float64{
		"A": 6, "B": 13, "C": 12, "D": 12, "E": 14,
		"F": 8, "G": 8, "H": 8, "I": 8, "J": 8,
		"K": 20, "L": 15, "M": 15, "N": 40,
	}
	for col, w := range widths {
		_ = f.SetColWidth(sheet, col, col, w)
	}

	// 2. Attach Header Logo (A2 - top left header)
	if len(assets.SDDLogo) > 0 {
		_ = addHeaderLogo(f, sheet, "A2", assets.SDDLogo, ".jpg", 0.6, 0.6)
	}

	// 3. Title (B1)
	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 14, Family: "Calibri"},
	})
	_ = f.SetCellValue(sheet, "B1", "ABSENSI MANUAL")
	_ = f.SetCellStyle(sheet, "B1", "B1", titleStyle)

	// 4. Metadata (Rows 2-7)
	setMeta := func(cellLabel, label, cellVal, val string) {
		_ = f.SetCellValue(sheet, cellLabel, label)
		_ = f.SetCellStyle(sheet, cellLabel, cellLabel, st.MetaLabelStyle)
		_ = f.SetCellValue(sheet, cellVal, val)
		_ = f.SetCellStyle(sheet, cellVal, cellVal, st.MetaValueStyle)
	}

	empID := in.User.EmployeeID
	if empID == "" {
		empID = in.User.MiiID
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

	// 5. Legend (Row 9)
	legends := map[string]string{
		"F9": "H : Hadir", "G9": "C = Cuti", "H9": "I=Izin", "I9": "S = Sakit", "J9": "L = Lembur",
	}
	for cell, leg := range legends {
		_ = f.SetCellValue(sheet, cell, leg)
		_ = f.SetCellStyle(sheet, cell, cell, st.BoldLeftStyle)
	}

	// 6. Table Headers (Rows 10-11)
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

	// 7. Daily Rows (Days 1 to 31, starting Row 12)
	byDay := make(map[int]models.DailyActivity, len(in.Activities))
	for _, a := range in.Activities {
		byDay[a.Date.Day()] = a
	}

	daysInMonth := GetDaysInMonth(in.Year, in.Month)
	const firstRow = 12

	for day := 1; day <= 31; day++ {
		row := firstRow + (day - 1)
		rs := fmt.Sprintf("%d", row)

		if day > daysInMonth {
			// Days beyond month length are left blank with borders
			for _, col := range []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N"} {
				_ = f.SetCellStyle(sheet, col+rs, col+rs, st.DataCenterStyle)
			}
			_ = f.SetCellStyle(sheet, "N"+rs, "N"+rs, st.DataCenterWrapStyle)
			_ = f.SetRowHeight(sheet, row, 15)
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

		for _, col := range []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N"} {
			_ = f.SetCellStyle(sheet, col+rs, col+rs, centerStyle)
		}
		_ = f.SetCellStyle(sheet, "N"+rs, "N"+rs, centerWrapStyle)

		_ = f.SetCellValue(sheet, "A"+rs, day)
		_ = f.SetCellValue(sheet, "B"+rs, date)
		_ = f.SetCellStyle(sheet, "B"+rs, "B"+rs, dateStyle)

		if hasAct {
			hasStart, hasEnd := false, false
			if act.StartTime != "" {
				if frac, ferr := parseTimeToExcelFraction(act.StartTime); ferr == nil {
					_ = f.SetCellValue(sheet, "C"+rs, frac)
					_ = f.SetCellStyle(sheet, "C"+rs, "C"+rs, timeStyle)
					hasStart = true
				}
			}
			if act.EndTime != "" {
				if frac, ferr := parseTimeToExcelFraction(act.EndTime); ferr == nil {
					_ = f.SetCellValue(sheet, "D"+rs, frac)
					_ = f.SetCellStyle(sheet, "D"+rs, "D"+rs, timeStyle)
					hasEnd = true
				}
			}

			// Formula total jam kerja = D - C
			if hasStart && hasEnd {
				_ = f.SetCellFormula(sheet, "E"+rs, fmt.Sprintf("D%s-C%s", rs, rs))
				_ = f.SetCellStyle(sheet, "E"+rs, "E"+rs, timeStyle)
			}

			// Mapping Status SDD: F=Hadir, G=Cuti, H=Izin, I=Sakit, J=Lembur
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

			_ = f.SetCellValue(sheet, "K"+rs, act.ProjectName)
			_ = f.SetCellValue(sheet, "L"+rs, act.ProjectID)
			_ = f.SetCellValue(sheet, "N"+rs, act.Activity)

			h := calculateRowHeight(act.Activity, act.ProjectName, act.ProjectID, "", "", "")
			_ = f.SetRowHeight(sheet, row, h)
		} else if isHolidayOrWeekend {
			if holiday != "" {
				_ = f.SetCellValue(sheet, "N"+rs, holiday)
			} else {
				_ = f.SetCellValue(sheet, "N"+rs, "Weekend")
			}
			_ = f.SetRowHeight(sheet, row, 15)
		}
	}

	// 8. Summary Row (Row 43) with COUNTIF formulas
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

	// 9. Signatures Area (Rows 46-52) matching official SDD template
	// Headers (Row 46)
	styleMergedRange(f, sheet, "C46", "G46", st.BoldCenterStyle)
	_ = f.SetCellValue(sheet, "C46", "Pemohon")

	styleMergedRange(f, sheet, "H46", "K46", st.BoldCenterStyle)
	_ = f.SetCellValue(sheet, "H46", "Diperiksa,")

	styleMergedRange(f, sheet, "L46", "M46", st.BoldCenterStyle)
	_ = f.SetCellValue(sheet, "L46", "Disetujui,")

	// Empty Signature Boxes with Borders (Rows 47-51)
	styleMergedRange(f, sheet, "C47", "G51", st.DataCenterStyle)
	styleMergedRange(f, sheet, "H47", "K51", st.DataCenterStyle)
	styleMergedRange(f, sheet, "L47", "M51", st.DataCenterStyle)

	// Names (Row 52)
	styleMergedRange(f, sheet, "C52", "G52", st.DataLeftStyle)
	_ = f.SetCellValue(sheet, "C52", "Nama : "+in.User.Name)

	styleMergedRange(f, sheet, "H52", "K52", st.DataLeftStyle)
	_ = f.SetCellValue(sheet, "H52", "Nama : ")

	styleMergedRange(f, sheet, "L52", "M52", st.DataLeftStyle)
	_ = f.SetCellValue(sheet, "L52", "Nama : ")

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
