package services

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"timesheet-backend/models"
)

// ParseXLSXGrid reads the given sheet of an uploaded template into a 2-D string
// grid suitable for rendering in Handsontable on the admin template builder.
// When sheetName is empty the first sheet is used.
func ParseXLSXGrid(data []byte, sheetName string) (grid [][]string, resolvedSheet string, err error) {
	layout, err := ParseXLSXLayout(data, sheetName)
	if err != nil {
		return nil, "", err
	}
	return layout.Grid, layout.SheetName, nil
}

// MergeRegion is a merged-cell region expressed in 0-based Handsontable
// coordinates with spans.
type MergeRegion struct {
	Row     int `json:"row"`
	Col     int `json:"col"`
	Rowspan int `json:"rowspan"`
	Colspan int `json:"colspan"`
}

// SheetLayout is a faithful-enough rendering of a template sheet for the admin
// mapping grid: values plus the merged-cell regions and column widths that
// define the layout. Cell styles and embedded images are NOT reproduced here —
// they don't affect mapping — but they ARE preserved in generated output, which
// injects data into a copy of the original uploaded file.
type SheetLayout struct {
	SheetName string        `json:"sheet_name"`
	Grid      [][]string    `json:"grid"`
	Merges    []MergeRegion `json:"merges"`
	ColWidths []int         `json:"col_widths"` // pixels, per column
}

// ParseXLSXLayout parses a template sheet into a SheetLayout, preserving merged
// regions and column widths so the Handsontable preview resembles the original.
// All rows are padded into a complete rectangular 2D matrix so every cell in the
// template grid is rendered and selectable in Handsontable.
func ParseXLSXLayout(data []byte, sheetName string) (*SheetLayout, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if sheetName == "" {
		sheetName = f.GetSheetName(0)
	}
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, err
	}

	layout := &SheetLayout{SheetName: sheetName}

	// Parse merged cells and keep track of max row & col indices.
	maxCols := 0
	maxRows := len(rows)

	for _, r := range rows {
		if len(r) > maxCols {
			maxCols = len(r)
		}
	}

	if dim, err := f.GetSheetDimension(sheetName); err == nil && dim != "" {
		parts := strings.Split(dim, ":")
		endAxis := parts[len(parts)-1]
		if col, row, err := excelize.CellNameToCoordinates(endAxis); err == nil {
			if col > maxCols {
				maxCols = col
			}
			if row > maxRows {
				maxRows = row
			}
		}
	}

	if merges, err := f.GetMergeCells(sheetName); err == nil {
		for _, m := range merges {
			sc, sr, e1 := excelize.CellNameToCoordinates(m.GetStartAxis())
			ec, er, e2 := excelize.CellNameToCoordinates(m.GetEndAxis())
			if e1 != nil || e2 != nil {
				continue
			}
			if ec > maxCols {
				maxCols = ec
			}
			if er > maxRows {
				maxRows = er
			}
			layout.Merges = append(layout.Merges, MergeRegion{
				Row:     sr - 1,
				Col:     sc - 1,
				Rowspan: er - sr + 1,
				Colspan: ec - sc + 1,
			})
		}
	}

	// Build a complete rectangular 2-D matrix (maxRows x maxCols) so no cells are omitted.
	grid := make([][]string, maxRows)
	for r := 0; r < maxRows; r++ {
		rowSlice := make([]string, maxCols)
		if r < len(rows) {
			for c := 0; c < len(rows[r]) && c < maxCols; c++ {
				rowSlice[c] = rows[r][c]
			}
		}
		grid[r] = rowSlice
	}
	layout.Grid = grid

	// Column widths (convert Excel character-width units to approx pixels).
	for col := 1; col <= maxCols; col++ {
		name, _ := excelize.ColumnNumberToName(col)
		w, err := f.GetColWidth(sheetName, name)
		if err != nil || w <= 0 {
			w = 8.43 // Excel default
		}
		layout.ColWidths = append(layout.ColWidths, int(w*7)+8)
	}

	return layout, nil
}

// generationInput bundles everything needed to render a user's monthly file.
type GenerationInput struct {
	Template   *models.Template
	Mappings   []models.CellMapping
	User       *models.User
	Month      int
	Year       int
	Activities []models.DailyActivity
	// Holidays maps day-of-month to a public-holiday name for the month.
	Holidays map[int]string
}

// GenerateFromTemplate injects a user's stored activities into the admin-mapped
// cells of a template and returns the resulting .xlsx bytes. Rows for days
// beyond the month's length are cleared so client templates that ship with a
// fixed 31-row block are trimmed to the correct length.
func GenerateFromTemplate(in GenerationInput) ([]byte, error) {
	// Built-in templates with a fixed layout use a dedicated strict-typed path.
	switch in.Template.Builtin {
	case "mii":
		return generateMII(in)
	case "sdd":
		return generateSDD(in)
	case "adidata":
		return generateAdidata(in)
	case "bni_dev":
		return generateBNI(in)
	}

	f, err := excelize.OpenReader(bytes.NewReader(in.Template.FileData))
	if err != nil {
		return nil, fmt.Errorf("open template: %w", err)
	}
	defer f.Close()

	sheet := in.Template.SheetName
	if sheet == "" {
		sheet = f.GetSheetName(0)
	}

	// Index activities by day-of-month for O(1) lookup.
	byDay := make(map[int]models.DailyActivity, len(in.Activities))
	for _, a := range in.Activities {
		byDay[a.Date.Day()] = a
	}

	daysInMonth := GetDaysInMonth(in.Year, in.Month)

	// Split mappings by scope.
	var cellMaps []models.CellMapping
	var dailyMaps []models.CellMapping
	for _, m := range in.Mappings {
		switch m.Scope {
		case models.ScopeDailyColumn:
			dailyMaps = append(dailyMaps, m)
		default:
			cellMaps = append(cellMaps, m)
		}
	}

	// 1. Static metadata cells.
	for _, m := range cellMaps {
		if m.CellRef == "" {
			continue
		}
		if v := metaValue(m.Field, in); v != "" {
			if err := f.SetCellValue(sheet, m.CellRef, v); err != nil {
				return nil, fmt.Errorf("set meta cell %s: %w", m.CellRef, err)
			}
		}
	}

	// 2. Daily columns: write one row per calendar day, trim the rest.
	// Track the maximum StartRow so we know where the block ends for trimming.
	maxTemplateRow := 0
	for _, m := range dailyMaps {
		if m.StartRow > maxTemplateRow {
			maxTemplateRow = m.StartRow
		}
	}

	for _, m := range dailyMaps {
		if m.Column == "" || m.StartRow == 0 {
			continue
		}
		for day := 1; day <= 31; day++ {
			row := m.StartRow + (day - 1)
			cell := fmt.Sprintf("%s%d", m.Column, row)
			if day > daysInMonth {
				// Trim excess rows for short months.
				_ = f.SetCellValue(sheet, cell, "")
				continue
			}
			act, ok := byDay[day]
			date := time.Date(in.Year, time.Month(in.Month), day, 0, 0, 0, 0, time.UTC)
			isWeekend := date.Weekday() == time.Saturday || date.Weekday() == time.Sunday
			holiday := in.Holidays[day]
			value := dailyValue(m.Field, act, ok, in.Year, in.Month, day, isWeekend, holiday)
			if err := f.SetCellValue(sheet, cell, value); err != nil {
				return nil, fmt.Errorf("set daily cell %s: %w", cell, err)
			}
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// MII layout hardcoded metadata. These are fixed by the client and written on
// every worked day of the MII template.
const (
	miiProjectName = "BNI Direct"
	miiProjectID   = "P24015"
	miiDivision    = "Wholesale Digital Delivery"
	miiDepartment  = "Wholesale Channel and Service Delivery"
)

// MIIAppImpactedOptions are the only accepted values for the MII "Aplikasi
// Terdampak" (application impacted) column.
var MIIAppImpactedOptions = []string{"Bisnis", "Cash", "Overseas"}

// NormalizeMIIAppImpacted maps a user-entered value to its canonical MII option
// (case-insensitively). It returns the canonical string when it matches one of
// the allowed options, otherwise the trimmed input is returned unchanged so no
// data is silently dropped.
func NormalizeMIIAppImpacted(v string) string {
	t := strings.TrimSpace(v)
	for _, opt := range MIIAppImpactedOptions {
		if strings.EqualFold(t, opt) {
			return opt
		}
	}
	return t
}

// generateMII renders the built-in "MII Timesheet" template. Its layout is
// fixed by the client file, so cells are addressed directly with strict typing:
//
//	C1     project name (hardcoded "BNI Direct")
//	C2-C6  header metadata (division/name/MII-ID/site from the profile; period)
//	A9:A39 date (d-mmm-yy)      B9:B39/C9:C39 start/end time (h:mm)
//	D9:D39 total hour (=C-B)    E9:J39 status matrix (E=P F=S G=BT H=PM I=V J=X)
//	K      activity/remark      N       aplikasi terdampak (Bisnis/Cash/Overseas)
//	L/M    project name/id (hardcoded)  P/Q  divisi/departement (hardcoded)
//	O/R    AIP fitur / sub-departement (left blank)
//	A43    employee signature (reviewer/approver blocks are left for hand-sign)
//
// Rows beyond the month's length are cleared so the COUNTIF totals stay correct.
func generateMII(in GenerationInput) ([]byte, error) {
	f, err := excelize.OpenReader(bytes.NewReader(in.Template.FileData))
	if err != nil {
		return nil, fmt.Errorf("open template: %w", err)
	}
	defer f.Close()

	sheet := in.Template.SheetName
	if sheet == "" {
		sheet = f.GetSheetName(0)
	}

	// setTyped writes a value while preserving the cell's existing number format.
	setTyped := func(cell string, v interface{}) {
		style, _ := f.GetCellStyle(sheet, cell)
		_ = f.SetCellValue(sheet, cell, v)
		_ = f.SetCellStyle(sheet, cell, cell, style)
	}
	setStr := func(cell, v string) { _ = f.SetCellValue(sheet, cell, v) }
	setFormula := func(cell, formula string) {
		style, _ := f.GetCellStyle(sheet, cell)
		_ = f.SetCellFormula(sheet, cell, formula)
		_ = f.SetCellStyle(sheet, cell, cell, style)
	}

	// --- Header metadata (column C), keeping the leading ": " prefix. ---
	setTyped("C1", ": "+miiProjectName)
	if in.User.Division != "" {
		setTyped("C2", ": "+in.User.Division)
	}
	if in.User.Name != "" {
		setTyped("C3", ": "+in.User.Name)
	}
	if in.User.MiiID != "" {
		setTyped("C4", ": "+in.User.MiiID)
	}
	if in.User.Site != "" {
		setTyped("C5", ": "+in.User.Site)
	}
	setTyped("C6", time.Date(in.Year, time.Month(in.Month), 1, 0, 0, 0, 0, time.UTC))

	// --- Employee signature block (merged A43:C46). ---
	if in.User.Name != "" {
		setTyped("A43", "( "+in.User.Name+" )")
	}

	byDay := make(map[int]models.DailyActivity, len(in.Activities))
	for _, a := range in.Activities {
		byDay[a.Date.Day()] = a
	}
	statusCol := map[string]string{"P": "E", "S": "F", "BT": "G", "PM": "H", "V": "I", "X": "J"}
	statusMark := map[string]string{"P": "P", "S": "S", "BT": "BT", "PM": "PM", "V": "V", "X": "x"}
	// Per-day data columns (everything except the date column A). The MII
	// template ships every data row pre-filled with placeholder hours, so these
	// are cleared at the start of each row before (re)writing.
	dataCols := []string{"B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R"}
	// All per-day columns, used to clear trimmed rows entirely.
	allCols := append([]string{"A"}, dataCols...)

	const firstRow = 9 // day 1
	daysInMonth := GetDaysInMonth(in.Year, in.Month)

	for day := 1; day <= 31; day++ {
		rs := fmt.Sprintf("%d", firstRow+(day-1))

		// Trim rows beyond the month's length.
		if day > daysInMonth {
			for _, col := range allCols {
				setStr(col+rs, "")
			}
			continue
		}

		date := time.Date(in.Year, time.Month(in.Month), day, 0, 0, 0, 0, time.UTC)
		setTyped("A"+rs, date)

		// Clear all per-day data cells first so the template's placeholder values
		// (it ships every row pre-filled with 8:00-17:00) never leak into
		// weekends, holidays, or working days the user hasn't filled in.
		for _, col := range dataCols {
			setStr(col+rs, "")
		}

		isWeekend := date.Weekday() == time.Saturday || date.Weekday() == time.Sunday
		holiday := in.Holidays[day]
		act, hasAct := byDay[day]

		status := ""
		if hasAct {
			status = strings.ToUpper(strings.TrimSpace(act.Status))

			hasStart, hasEnd := false, false
			if act.StartTime != "" {
				if frac, ferr := parseTimeToExcelFraction(act.StartTime); ferr == nil {
					setTyped("B"+rs, frac)
					hasStart = true
				}
			}
			if act.EndTime != "" {
				if frac, ferr := parseTimeToExcelFraction(act.EndTime); ferr == nil {
					setTyped("C"+rs, frac)
					hasEnd = true
				}
			}
			// Total hour = End - Start (only when both are present).
			if hasStart && hasEnd {
				setFormula("D"+rs, fmt.Sprintf("C%s-B%s", rs, rs))
			}

			setStr("K"+rs, act.Activity)
			setStr("L"+rs, miiProjectName)
			setStr("M"+rs, miiProjectID)
			setStr("N"+rs, NormalizeMIIAppImpacted(act.AppImpacted))
			setStr("O"+rs, "") // AIP Fitur (blank)
			setStr("P"+rs, miiDivision)
			setStr("Q"+rs, miiDepartment)
			setStr("R"+rs, "") // Sub Departement (blank)
		} else if isWeekend || holiday != "" {
			status = "X"
			if holiday != "" {
				setStr("K"+rs, holiday)
			} else {
				setStr("K"+rs, "Weekend")
			}
			// (metadata columns already cleared above)
		}

		if col, ok := statusCol[status]; ok {
			setStr(col+rs, statusMark[status])
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// generateBNI renders the built-in "BNI DEV Timesheet" template. Its layout is
// fixed by the client file, so cells are addressed directly with strict typing:
//
//	C1-C6  header metadata (kept behind the ": " prefix; period is a real date)
//	A9:A39 date (d-mmm-yy)   B9:B39/C9:C39 start/end time (h:mm)
//	E9:J39 status matrix (one column per status: E=P F=S G=BT H=PM I=V J=x)
//	K/L/M/N per-day activity/project/id/app, P divisi from the user profile
//	A43    employee signature (reviewer/approver blocks are left for hand-sign)
//
// Rows beyond the month's length are cleared so the COUNTIF totals stay correct.
func generateBNI(in GenerationInput) ([]byte, error) {
	f, err := excelize.OpenReader(bytes.NewReader(in.Template.FileData))
	if err != nil {
		return nil, fmt.Errorf("open template: %w", err)
	}
	defer f.Close()

	sheet := in.Template.SheetName
	if sheet == "" {
		sheet = f.GetSheetName(0)
	}

	// setTyped writes a value while preserving the cell's existing number format
	// (dates stay d-mmm-yy, times stay h:mm, period stays mmm-yy).
	setTyped := func(cell string, v interface{}) {
		style, _ := f.GetCellStyle(sheet, cell)
		_ = f.SetCellValue(sheet, cell, v)
		_ = f.SetCellStyle(sheet, cell, cell, style)
	}
	setStr := func(cell, v string) { _ = f.SetCellValue(sheet, cell, v) }

	// --- Header metadata (column C), keeping the leading ": " prefix. ---
	if in.User.Division != "" {
		setTyped("C2", ": "+in.User.Division)
	}
	if in.User.Name != "" {
		setTyped("C3", ": "+in.User.Name)
	}
	if in.User.MiiID != "" {
		setTyped("C4", ": "+in.User.MiiID)
	}
	if in.User.Site != "" {
		setTyped("C5", ": "+in.User.Site)
	}
	setTyped("C6", time.Date(in.Year, time.Month(in.Month), 1, 0, 0, 0, 0, time.UTC))

	// --- Employee signature block (merged A43:C46). ---
	if in.User.Name != "" {
		setTyped("A43", "( "+in.User.Name+" )")
	}

	byDay := make(map[int]models.DailyActivity, len(in.Activities))
	for _, a := range in.Activities {
		byDay[a.Date.Day()] = a
	}
	statusCol := map[string]string{"P": "E", "S": "F", "BT": "G", "PM": "H", "V": "I", "X": "J"}
	statusMark := map[string]string{"P": "P", "S": "S", "BT": "BT", "PM": "PM", "V": "V", "X": "x"}
	matrixCols := []string{"E", "F", "G", "H", "I", "J"}

	const firstRow = 9 // day 1
	daysInMonth := GetDaysInMonth(in.Year, in.Month)

	for day := 1; day <= 31; day++ {
		rs := fmt.Sprintf("%d", firstRow+(day-1))

		// Trim rows beyond the month's length.
		if day > daysInMonth {
			for _, col := range []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R"} {
				setStr(col+rs, "")
			}
			continue
		}

		date := time.Date(in.Year, time.Month(in.Month), day, 0, 0, 0, 0, time.UTC)
		setTyped("A"+rs, date)

		// Reset the status matrix for the row before marking it.
		for _, col := range matrixCols {
			setStr(col+rs, "")
		}

		isWeekend := date.Weekday() == time.Saturday || date.Weekday() == time.Sunday
		holiday := in.Holidays[day]
		act, hasAct := byDay[day]

		status := ""
		if hasAct {
			status = strings.ToUpper(strings.TrimSpace(act.Status))
			if act.StartTime != "" {
				if frac, ferr := parseTimeToExcelFraction(act.StartTime); ferr == nil {
					setTyped("B"+rs, frac)
				}
			}
			if act.EndTime != "" {
				if frac, ferr := parseTimeToExcelFraction(act.EndTime); ferr == nil {
					setTyped("C"+rs, frac)
				}
			}
			setStr("K"+rs, act.Activity)
			setStr("L"+rs, act.ProjectName)
			setStr("M"+rs, act.ProjectID)
			setStr("N"+rs, act.AppImpacted)
			if in.User.Division != "" {
				setStr("P"+rs, in.User.Division)
			}
		} else if isWeekend || holiday != "" {
			status = "X"
			if holiday != "" {
				setStr("K"+rs, holiday)
			} else {
				setStr("K"+rs, "Weekend")
			}
		}

		if col, ok := statusCol[status]; ok {
			setStr(col+rs, statusMark[status])
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// metaValue resolves a static metadata field from the generation input.
func metaValue(field models.MappingFieldType, in GenerationInput) string {
	switch field {
	case models.FieldMetaName:
		return in.User.Name
	case models.FieldMetaMiiID:
		return in.User.MiiID
	case models.FieldMetaDivision:
		return in.User.Division
	case models.FieldMetaSite:
		return in.User.Site
	case models.FieldMetaMonth:
		return time.Month(in.Month).String()
	case models.FieldMetaYear:
		return fmt.Sprintf("%d", in.Year)
	}
	return ""
}

// dailyValue resolves a per-day column value for a given field and day. When a
// day has no user activity but is a weekend or public holiday, the status and
// activity cells are marked so the generated sheet reflects non-working days.
func dailyValue(field models.MappingFieldType, act models.DailyActivity, hasActivity bool, year, month, day int, isWeekend bool, holiday string) interface{} {
	switch field {
	case models.FieldDate:
		return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
	}
	if !hasActivity {
		if isWeekend || holiday != "" {
			switch field {
			case models.FieldStatus:
				return "X"
			case models.FieldActivity:
				if holiday != "" {
					return holiday
				}
				return "Weekend"
			}
		}
		return ""
	}
	switch field {
	case models.FieldTimeIn:
		return act.StartTime
	case models.FieldTimeOut:
		return act.EndTime
	case models.FieldStatus:
		return act.Status
	case models.FieldActivity:
		return act.Activity
	case models.FieldProjectName:
		return act.ProjectName
	case models.FieldProjectID:
		return act.ProjectID
	case models.FieldAppImpacted:
		return act.AppImpacted
	}
	return ""
}
