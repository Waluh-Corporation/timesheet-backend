package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"timesheet-backend/assets"
	"timesheet-backend/models"
)

// buildAdidataWorkbook generates the Adidata timesheet Excel document purely from code,
// including dynamic SPL sheets if overtime entries are present.
func buildAdidataWorkbook(in GenerationInput) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	const sheetTS = "TIMESHEET"
	_ = f.SetSheetName(f.GetSheetName(0), sheetTS)

	st, err := NewBuilderStyles(f)
	if err != nil {
		return nil, fmt.Errorf("create builder styles: %w", err)
	}

	// 1. Column Widths
	widths := map[string]float64{
		"A": 12, "B": 8, "C": 8, "D": 11,
		"E": 8, "F": 8, "G": 13, "H": 8, "I": 8, "J": 12,
		"K": 35, "L": 18, "M": 14, "N": 18, "O": 14,
	}
	for col, w := range widths {
		_ = f.SetColWidth(sheetTS, col, col, w)
	}

	// 2. Attach Header Logo (N2 - top right header)
	if len(assets.AdidataLogo) > 0 {
		_ = addHeaderLogo(f, sheetTS, "N2", assets.AdidataLogo, ".png", 0.7, 0.7)
	}

	// 3. Metadata Header (Rows 1-5)
	setMeta := func(cellLabel, label, cellVal, val string) {
		_ = f.SetCellValue(sheetTS, cellLabel, label)
		_ = f.SetCellStyle(sheetTS, cellLabel, cellLabel, st.MetaLabelStyle)
		_ = f.SetCellValue(sheetTS, cellVal, val)
		_ = f.SetCellStyle(sheetTS, cellVal, cellVal, st.MetaValueStyle)
	}

	empID := in.User.EmployeeID
	if empID == "" {
		empID = in.User.BniID
	}
	div := in.User.Division
	if div == "" {
		div = "BDD / WDL"
	}
	periodStr := fmt.Sprintf("%s %d", indonesianMonth(in.Month), in.Year)

	setMeta("A1", "NAME of PROJECT", "C1", ": PT BANK NEGARA INDONESIA (PERSERO) Tbk")
	setMeta("A2", "UNIT/DIVISION", "C2", ": "+div)
	setMeta("A3", "NAME", "C3", ": "+in.User.Name)
	setMeta("A4", "NPP", "C4", ": "+empID)
	setMeta("A5", "PERIODE", "C5", ": "+periodStr)

	_ = f.SetCellValue(sheetTS, "G6", "P = Present; S = Sick; V = Vacation; BT = Business Trip; PM = Permit; X = Not Working Anymore")

	// 4. Table Headers (Rows 7-8)
	type colHeader struct {
		from, to string
		val      string
	}
	mergedHeaders := []colHeader{
		{"A7", "A8", "DATE"},
		{"B7", "C7", "WORKING HOUR"},
		{"D7", "D8", "TOTAL HOUR"},
		{"E7", "J7", "STATUS ATTENDANCE "},
		{"K7", "K8", "ACTIVITY / REMARK"},
		{"L7", "L8", "NAMA PROJECT"},
		{"M7", "M8", "PROJECT CODE"},
		{"N7", "N8", "APLIKASI TERDAMPAK"},
		{"O7", "O8", "AIP FITUR"},
	}
	for _, mh := range mergedHeaders {
		if mh.from != mh.to {
			_ = f.MergeCell(sheetTS, mh.from, mh.to)
		}
		_ = f.SetCellValue(sheetTS, mh.from, mh.val)
		_ = f.SetCellStyle(sheetTS, mh.from, mh.to, st.HeaderStyle)
	}

	subHeaders := map[string]string{
		"B8": "START", "C8": "END",
		"E8": "Present", "F8": "Sick ", "G8": "Business Trip",
		"H8": "Permit", "I8": "Vacation", "J8": "Not Working",
	}
	for cell, val := range subHeaders {
		_ = f.SetCellValue(sheetTS, cell, val)
		_ = f.SetCellStyle(sheetTS, cell, cell, st.HeaderGreyStyle)
	}

	// 5. Daily Rows (Days 1 to 31, starting Row 9)
	byDay := make(map[int]models.DailyActivity, len(in.Activities))
	for _, a := range in.Activities {
		byDay[a.Date.Day()] = a
	}

	statusCol := map[string]string{"P": "E", "S": "F", "BT": "G", "PM": "H", "V": "I", "X": "J"}
	statusMark := map[string]string{"P": "P", "S": "S", "BT": "BT", "PM": "PM", "V": "V", "X": "x"}
	matrixCols := []string{"E", "F", "G", "H", "I", "J"}

	daysInMonth := GetDaysInMonth(in.Year, in.Month)
	const firstRow = 9

	for day := 1; day <= 31; day++ {
		row := firstRow + (day - 1)
		rs := fmt.Sprintf("%d", row)

		if day > daysInMonth {
			for _, col := range []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O"} {
				_ = f.SetCellStyle(sheetTS, col+rs, col+rs, st.DataCenterStyle)
			}
			_ = f.SetCellStyle(sheetTS, "K"+rs, "K"+rs, st.DataCenterWrapStyle)
			_ = f.SetRowHeight(sheetTS, row, 15)
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
		decimalStyle := st.DecimalStyle
		if isHolidayOrWeekend {
			centerStyle = st.GreyCenterStyle
			centerWrapStyle = st.GreyCenterWrapStyle
			dateStyle = st.GreyDateStyle
			timeStyle = st.GreyTimeStyle
			decimalStyle = st.GreyDecimalStyle
		}

		for _, col := range []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O"} {
			_ = f.SetCellStyle(sheetTS, col+rs, col+rs, centerStyle)
		}
		_ = f.SetCellStyle(sheetTS, "K"+rs, "K"+rs, centerWrapStyle)

		_ = f.SetCellValue(sheetTS, "A"+rs, date)
		_ = f.SetCellStyle(sheetTS, "A"+rs, "A"+rs, dateStyle)

		status := ""
		if hasAct {
			status = strings.ToUpper(strings.TrimSpace(act.Status))
			hasStart, hasEnd := false, false

			if act.StartTime != "" {
				if frac, ferr := parseTimeToExcelFraction(act.StartTime); ferr == nil {
					_ = f.SetCellValue(sheetTS, "B"+rs, frac)
					_ = f.SetCellStyle(sheetTS, "B"+rs, "B"+rs, timeStyle)
					hasStart = true
				}
			}
			if act.EndTime != "" {
				if frac, ferr := parseTimeToExcelFraction(act.EndTime); ferr == nil {
					_ = f.SetCellValue(sheetTS, "C"+rs, frac)
					_ = f.SetCellStyle(sheetTS, "C"+rs, "C"+rs, timeStyle)
					hasEnd = true
				}
			}

			// Formula Adidata: Total hour = (C - B) * 24 (Decimal formatted as 0.00)
			if hasStart && hasEnd {
				_ = f.SetCellFormula(sheetTS, "D"+rs, fmt.Sprintf("(C%s-B%s)*24", rs, rs))
				_ = f.SetCellStyle(sheetTS, "D"+rs, "D"+rs, decimalStyle)
			}

			_ = f.SetCellValue(sheetTS, "K"+rs, act.Activity)
			_ = f.SetCellValue(sheetTS, "L"+rs, act.GetProjectName())
			_ = f.SetCellValue(sheetTS, "M"+rs, act.GetProjectCode())
			_ = f.SetCellValue(sheetTS, "N"+rs, act.GetAppImpacted())

			h := calculateRowHeight(act.Activity, act.GetProjectName(), act.GetProjectCode(), act.GetAppImpacted(), "", "")
			_ = f.SetRowHeight(sheetTS, row, h)
		} else if isHolidayOrWeekend {
			if holiday != "" {
				_ = f.SetCellValue(sheetTS, "K"+rs, holiday)
			} else {
				_ = f.SetCellValue(sheetTS, "K"+rs, "Weekend")
			}
			_ = f.SetRowHeight(sheetTS, row, 15)
		}

		for _, col := range matrixCols {
			_ = f.SetCellValue(sheetTS, col+rs, "")
		}
		if col, ok := statusCol[status]; ok {
			_ = f.SetCellValue(sheetTS, col+rs, statusMark[status])
		}
	}

	// 6. Summary Row (Row 40) with COUNTIF formulas
	const sumRow = "40"
	formulas := map[string]string{
		"E": `COUNTIF(E9:E39,"P")`,
		"F": `COUNTIF(F9:F39,"S")`,
		"G": `COUNTIF(G9:G39,"BT")`,
		"H": `COUNTIF(H9:H39,"PM")`,
		"I": `COUNTIF(I9:I39,"V")`,
		"J": `COUNTIF(J9:J39,"x")`,
	}
	for col, formula := range formulas {
		cell := col + sumRow
		_ = f.SetCellFormula(sheetTS, cell, formula)
		_ = f.SetCellStyle(sheetTS, cell, cell, st.BoldCenterStyle)
	}

	// 7. Signature Area (Rows 42-49) matching official Adidata template
	// Headers (Row 42)
	styleMergedRange(f, sheetTS, "A42", "C42", st.BoldCenterStyle)
	_ = f.SetCellValue(sheetTS, "A42", "TTD PEGAWAI,")

	styleMergedRange(f, sheetTS, "D42", "F42", st.BoldCenterStyle)
	_ = f.SetCellValue(sheetTS, "D42", "DIPERIKSA OLEH,")

	styleMergedRange(f, sheetTS, "G42", "J42", st.BoldCenterStyle)
	_ = f.SetCellValue(sheetTS, "G42", "DISETUJUI OLEH,")

	// Empty Signature Spaces with Borders (Rows 43-47)
	styleMergedRange(f, sheetTS, "A43", "C47", st.DataCenterStyle)
	styleMergedRange(f, sheetTS, "D43", "F47", st.DataCenterStyle)
	styleMergedRange(f, sheetTS, "G43", "J47", st.DataCenterStyle)

	// Names (Row 48)
	styleMergedRange(f, sheetTS, "A48", "C48", st.BoldCenterStyle)
	_ = f.SetCellValue(sheetTS, "A48", "( "+in.User.Name+" )")

	// Extract approver names from overtime entries if present
	tlName := ""
	dhName := ""
	for _, ot := range in.Overtimes {
		if ot.TeamLeader != nil && ot.TeamLeader.Name != "" && tlName == "" {
			tlName = ot.TeamLeader.Name
		}
		if ot.DepartmentHead != nil && ot.DepartmentHead.Name != "" && dhName == "" {
			dhName = ot.DepartmentHead.Name
		}
	}
	tlCell := ""
	if tlName != "" {
		tlCell = "( " + tlName + " )"
	}
	dhCell := ""
	if dhName != "" {
		dhCell = "( " + dhName + " )"
	}

	styleMergedRange(f, sheetTS, "D48", "F48", st.BoldCenterStyle)
	_ = f.SetCellValue(sheetTS, "D48", tlCell)

	styleMergedRange(f, sheetTS, "G48", "J48", st.BoldCenterStyle)
	_ = f.SetCellValue(sheetTS, "G48", dhCell)

	// Positions (Row 49)
	posTitle := in.User.Position
	if posTitle == "" {
		posTitle = "Junior Programmer"
	}
	styleMergedRange(f, sheetTS, "A49", "C49", st.BoldCenterStyle)
	_ = f.SetCellValue(sheetTS, "A49", posTitle)

	styleMergedRange(f, sheetTS, "D49", "F49", st.BoldCenterStyle)
	_ = f.SetCellValue(sheetTS, "D49", "TEAM LEADER")

	styleMergedRange(f, sheetTS, "G49", "J49", st.BoldCenterStyle)
	_ = f.SetCellValue(sheetTS, "G49", "DEPARTEMEN HEAD")

	// 8. Build SPL (Surat Perintah Lembur) sheets if overtime records exist
	for i, ot := range in.Overtimes {
		splSheet := fmt.Sprintf("SPL-%d", i+1)
		_, _ = f.NewSheet(splSheet)

		// Column Widths for SPL
		_ = f.SetColWidth(splSheet, "A", "A", 4)
		_ = f.SetColWidth(splSheet, "B", "B", 4)
		_ = f.SetColWidth(splSheet, "C", "C", 6)
		_ = f.SetColWidth(splSheet, "D", "E", 12)
		_ = f.SetColWidth(splSheet, "F", "G", 10)
		_ = f.SetColWidth(splSheet, "H", "O", 10)

		// Logo at L3 (header right)
		if len(assets.AdidataLogo) > 0 {
			_ = addHeaderLogo(f, splSheet, "L3", assets.AdidataLogo, ".png", 0.6, 0.6)
		}

		// Title B2
		titleStyle, _ := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Size: 14, Family: "Calibri"},
		})
		_ = f.SetCellValue(splSheet, "B2", "SURAT PERINTAH LEMBUR")
		_ = f.SetCellStyle(splSheet, "B2", "B2", titleStyle)

		// SPL Metadata (Rows 4-7)
		pos := in.User.Position
		if pos == "" {
			pos = "Junior Programmer"
		}
		deptDiv := in.User.Department
		if deptDiv == "" {
			deptDiv = "WCH / WDL"
		} else if in.User.Division != "" {
			deptDiv = deptDiv + " / " + in.User.Division
		}

		_ = f.SetCellValue(splSheet, "C4", "NPP")
		_ = f.SetCellValue(splSheet, "E4", ": "+empID)
		_ = f.SetCellValue(splSheet, "C5", "NAMA")
		_ = f.SetCellValue(splSheet, "E5", ": "+in.User.Name)
		_ = f.SetCellValue(splSheet, "C6", "DEPARTEMENT / DIVISI")
		_ = f.SetCellValue(splSheet, "E6", ": "+deptDiv)
		_ = f.SetCellValue(splSheet, "C7", "POSISI")
		_ = f.SetCellValue(splSheet, "E7", ": "+pos)

		for r := 4; r <= 7; r++ {
			_ = f.SetCellStyle(splSheet, fmt.Sprintf("C%d", r), fmt.Sprintf("C%d", r), st.MetaLabelStyle)
			_ = f.SetCellStyle(splSheet, fmt.Sprintf("E%d", r), fmt.Sprintf("E%d", r), st.MetaValueStyle)
		}

		// Table Header (Rows 9-11)
		_ = f.MergeCell(splSheet, "C9", "O9")
		_ = f.SetCellValue(splSheet, "C9", "TUGAS YANG DIKERJAKAN")
		_ = f.SetCellStyle(splSheet, "C9", "O9", st.HeaderStyle)

		_ = f.SetCellValue(splSheet, "C11", "No")
		_ = f.SetCellStyle(splSheet, "C11", "C11", st.HeaderStyle)

		_ = f.MergeCell(splSheet, "D11", "E11")
		_ = f.SetCellValue(splSheet, "D11", "Tanggal")
		_ = f.SetCellStyle(splSheet, "D11", "E11", st.HeaderStyle)

		_ = f.MergeCell(splSheet, "F11", "G11")
		_ = f.SetCellValue(splSheet, "F11", "Jam")
		_ = f.SetCellStyle(splSheet, "F11", "G11", st.HeaderStyle)

		_ = f.MergeCell(splSheet, "H11", "O11")
		_ = f.SetCellValue(splSheet, "H11", "Tugas yang di Kerjakan")
		_ = f.SetCellStyle(splSheet, "H11", "O11", st.HeaderStyle)

		// Row 12 (Task row)
		_ = f.SetCellValue(splSheet, "C12", "1")
		_ = f.SetCellStyle(splSheet, "C12", "C12", st.DataCenterStyle)

		_ = f.MergeCell(splSheet, "D12", "E12")
		_ = f.SetCellValue(splSheet, "D12", ot.Date.Format("02 January 2006"))
		_ = f.SetCellStyle(splSheet, "D12", "E12", st.DataCenterStyle)

		_ = f.MergeCell(splSheet, "F12", "G12")
		timeRange := fmt.Sprintf("%s - %s", ot.StartTime, ot.EndTime)
		_ = f.SetCellValue(splSheet, "F12", timeRange)
		_ = f.SetCellStyle(splSheet, "F12", "G12", st.DataCenterStyle)

		_ = f.MergeCell(splSheet, "H12", "O12")
		_ = f.SetCellValue(splSheet, "H12", ot.TaskDescription)
		_ = f.SetCellStyle(splSheet, "H12", "O12", st.DataCenterWrapStyle)

		// Signatures (Row 21-22)
		_ = f.MergeCell(splSheet, "C21", "E21")
		_ = f.SetCellValue(splSheet, "C21", "( "+in.User.Name+" )")
		_ = f.SetCellStyle(splSheet, "C21", "E21", st.BoldCenterStyle)

		_ = f.MergeCell(splSheet, "C22", "E22")
		_ = f.SetCellValue(splSheet, "C22", pos)
		_ = f.SetCellStyle(splSheet, "C22", "E22", st.DataCenterStyle)

		tl := ""
		if ot.TeamLeader != nil && ot.TeamLeader.Name != "" {
			tl = ot.TeamLeader.Name
		}
		tlLabel := ""
		if tl != "" {
			tlLabel = "( " + tl + " )"
		}
		_ = f.MergeCell(splSheet, "G21", "J21")
		_ = f.SetCellValue(splSheet, "G21", tlLabel)
		_ = f.SetCellStyle(splSheet, "G21", "J21", st.BoldCenterStyle)

		_ = f.MergeCell(splSheet, "G22", "J22")
		_ = f.SetCellValue(splSheet, "G22", "TEAM LEADER")
		_ = f.SetCellStyle(splSheet, "G22", "J22", st.DataCenterStyle)

		dh := ""
		if ot.DepartmentHead != nil && ot.DepartmentHead.Name != "" {
			dh = ot.DepartmentHead.Name
		}
		dhLabel := ""
		if dh != "" {
			dhLabel = "( " + dh + " )"
		}
		_ = f.MergeCell(splSheet, "L21", "N21")
		_ = f.SetCellValue(splSheet, "L21", dhLabel)
		_ = f.SetCellStyle(splSheet, "L21", "N21", st.BoldCenterStyle)

		_ = f.MergeCell(splSheet, "L22", "N22")
		_ = f.SetCellValue(splSheet, "L22", "DEPARTMENT HEAD")
		_ = f.SetCellStyle(splSheet, "L22", "N22", st.DataCenterStyle)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
