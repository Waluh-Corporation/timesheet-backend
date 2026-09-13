package services

import (
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"

	"timesheet-backend/assets"
	"timesheet-backend/models"
)

const nttDatePrefix = "DATE: "

func setNTTColWidths(f *excelize.File, sheet string) {
	widths := map[string]float64{
		"A": 12, "B": 8, "C": 8, "D": 11,
		"E": 8, "F": 8, "G": 13, "H": 8, "I": 8, "J": 12,
		"K": 35, "L": 18, "M": 14, "N": 25,
	}
	for col, w := range widths {
		_ = f.SetColWidth(sheet, col, col, w)
	}
}

func writeNTTMetadata(f *excelize.File, sheet string, in GenerationInput, st *BuilderStyles) {
	empID := ResolveEmployeeID(in.User)
	div := in.User.Division
	if div == "" {
		div = "WDL"
	}
	periodStr := fmt.Sprintf("%s-%02d", time.Month(in.Month).String()[:3], in.Year%100)

	WriteMetaField(f, sheet, "A4", "NAME of PROJECT", "B4", ": BNIdirect", st)
	WriteMetaField(f, sheet, "A5", "UNIT / DIVISION", "B5", ": "+div, st)
	WriteMetaField(f, sheet, "A6", "NAME", "B6", ": "+in.User.Name, st)
	WriteMetaField(f, sheet, "A7", "NTT ID", "B7", ": "+empID, st)
	_ = f.SetCellValue(sheet, "J7", ": Holiday")

	WriteMetaField(f, sheet, "A8", "PERIODE", "B8", ": "+periodStr, st)
	_ = f.SetCellValue(sheet, "I8", "P = Present; S = Sick;  V = Vacation; BT = Business Trip; PM = Permit; X = Not Working Anymore")
}

func writeNTTHeaders(f *excelize.File, sheet string, st *BuilderStyles) {
	headers := []HeaderColumn{
		{From: "A9", To: "A10", Val: "DATE"},
		{From: "B9", To: "C9", Val: "WORKING HOUR"},
		{From: "D9", To: "D10", Val: "TOTAL HOUR"},
		{From: "E9", To: "J9", Val: "STATUS  ATTENDANCE"},
		{From: "K9", To: "K10", Val: "ACTIVITY / REMARK"},
		{From: "L9", To: "L10", Val: "Project Name"},
		{From: "M9", To: "M10", Val: "Project Code"},
		{From: "N9", To: "N10", Val: "Application Name"},
	}
	subHeaders := map[string]string{
		"B10": "START", "C10": "END",
		"E10": "Present", "F10": "Sick", "G10": "Business Trip",
		"H10": "Permit", "I10": "Vacation", "J10": "Not Working",
	}
	WriteTableHeaders(f, sheet, headers, subHeaders, st)
}

func writeNTTActRow(f *excelize.File, sheet, rs string, row int, act models.DailyActivity, timeStyle int) {
	hasStart, hasEnd := WriteTimeCells(f, sheet, "B"+rs, "C"+rs, act.StartTime, act.EndTime, timeStyle)
	if hasStart && hasEnd {
		_ = f.SetCellFormula(sheet, "D"+rs, fmt.Sprintf("IF(C%s>B%s,(C%s-B%s),C%s-B%s+1)", rs, rs, rs, rs, rs, rs))
		_ = f.SetCellStyle(sheet, "D"+rs, "D"+rs, timeStyle)
	}

	projName := act.GetProjectName()
	if projName == "" {
		projName = "BNI Direct"
	}
	projCode := act.GetProjectCode()
	if projCode == "" {
		projCode = "P24015"
	}
	WriteActivityProjectCells(f, sheet, rs, row, act.Activity, projName, projCode, act.GetAppImpacted())
}

func writeNTTDailyRows(f *excelize.File, sheet string, in GenerationInput, st *BuilderStyles) {
	allCols := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N"}
	WriteBuilderDailyRows(f, sheet, in, 11, allCols, []string{"K"}, st, func(rs string, row int, dsc DayStyleContext) {
		writeNTTActRow(f, sheet, rs, row, dsc.Activity, dsc.TimeStyle)
	})
}

func writeNTTProjectSummary(f *excelize.File, sheet string, st *BuilderStyles) {
	formulas := map[string]string{
		"E": `COUNTA(E11:E41)`,
		"F": `COUNTA(F11:F41)`,
		"G": `COUNTA(G11:G41)`,
		"H": `COUNTA(H11:H41)`,
		"I": `COUNTA(I11:I41)`,
		"J": `COUNTA(J11:J41)`,
	}
	WriteColumnFormulas(f, sheet, "42", formulas, st.BoldCenterStyle)

	_ = f.SetCellValue(sheet, "A44", "No.")
	_ = f.SetCellStyle(sheet, "A44", "A45", st.HeaderStyle)
	_ = f.MergeCell(sheet, "A44", "A45")

	_ = f.MergeCell(sheet, "B44", "D44")
	_ = f.SetCellValue(sheet, "B44", "Project Code / Name")
	_ = f.SetCellStyle(sheet, "B44", "D45", st.HeaderStyle)
	_ = f.MergeCell(sheet, "B44", "D45")

	_ = f.MergeCell(sheet, "E44", "J44")
	_ = f.SetCellValue(sheet, "E44", "STATUS  ATTENDANCE")
	_ = f.SetCellStyle(sheet, "E44", "J44", st.HeaderStyle)

	subProjHeaders := map[string]string{
		"E45": "Present\n(Days)", "F45": "Sick\n(Days)", "G45": "Business Trip\n(Days)",
		"H45": "Permit\n(Days)", "I45": "Vacation\n(Days)", "J45": "Not Working\n(Days)",
	}
	for cell, val := range subProjHeaders {
		_ = f.SetCellValue(sheet, cell, val)
		_ = f.SetCellStyle(sheet, cell, cell, st.HeaderGreyStyle)
	}

	_ = f.MergeCell(sheet, "K44", "K45")
	_ = f.SetCellValue(sheet, "K44", "Total Days")
	_ = f.SetCellStyle(sheet, "K44", "K45", st.HeaderStyle)

	_ = f.MergeCell(sheet, "L44", "L45")
	_ = f.SetCellValue(sheet, "L44", "Status (CR done/ in progress)")
	_ = f.SetCellStyle(sheet, "L44", "L45", st.HeaderStyle)

	_ = f.SetCellValue(sheet, "A46", 1)
	_ = f.SetCellStyle(sheet, "A46", "A46", st.DataCenterStyle)
	_ = f.MergeCell(sheet, "B46", "D46")
	_ = f.SetCellValue(sheet, "B46", "P24015 - BNI Direct")
	_ = f.SetCellStyle(sheet, "B46", "D46", st.DataLeftStyle)

	for _, col := range []string{"E", "F", "G", "H", "I", "J"} {
		_ = f.SetCellFormula(sheet, col+"46", col+"42")
		_ = f.SetCellStyle(sheet, col+"46", col+"46", st.DataCenterStyle)
	}
	_ = f.SetCellFormula(sheet, "K46", "SUM(E46:J46)")
	_ = f.SetCellStyle(sheet, "K46", "K46", st.BoldCenterStyle)
	_ = f.SetCellValue(sheet, "L46", "In Progress")
	_ = f.SetCellStyle(sheet, "L46", "L46", st.DataCenterStyle)

	_ = f.MergeCell(sheet, "A51", "D51")
	_ = f.SetCellValue(sheet, "A51", "TOTAL")
	_ = f.SetCellStyle(sheet, "A51", "D51", st.BoldCenterStyle)

	sumFormulas := make(map[string]string)
	for _, col := range []string{"E", "F", "G", "H", "I", "J", "K"} {
		sumFormulas[col] = fmt.Sprintf("SUM(%s46:%s50)", col, col)
	}
	WriteColumnFormulas(f, sheet, "51", sumFormulas, st.BoldCenterStyle)
}

func writeNTTSignatures(f *excelize.File, sheet string, in GenerationInput, st *BuilderStyles) {
	tlName, dhName := ResolveApprovers(in)

	WriteSignaturesLayout(f, sheet, 53, 3, []SignatureParty{
		{StartCol: "B", EndCol: "E", Title: "TTD PEGAWAI,", Name: in.User.Name, DatePrefix: nttDatePrefix},
		{StartCol: "F", EndCol: "I", Title: "DIPERIKSA OLEH,", Name: tlName, DatePrefix: nttDatePrefix},
		{StartCol: "J", EndCol: "L", Title: "DISETUJUI OLEH,", Name: dhName, DatePrefix: nttDatePrefix},
	}, st)

	_ = f.SetRowHeight(sheet, 53, 20)
	_ = f.SetRowHeight(sheet, 54, 16)
	_ = f.SetRowHeight(sheet, 55, 16)
	_ = f.SetRowHeight(sheet, 56, 18)
	_ = f.SetRowHeight(sheet, 57, 22)
	_ = f.SetRowHeight(sheet, 58, 18)
}

// buildNTTWorkbook generates the NTT timesheet Excel document purely from code.
func buildNTTWorkbook(in GenerationInput) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	const sheet = "Timesheet"
	_ = f.SetSheetName(f.GetSheetName(0), sheet)
	userName := ""
	if in.User != nil {
		userName = in.User.Name
	}
	SetWorkbookProperties(f, "Timesheet NTT", userName)

	st, err := NewBuilderStyles(f)
	if err != nil {
		return nil, fmt.Errorf("create builder styles: %w", err)
	}

	setNTTColWidths(f, sheet)

	if len(assets.NTTLogo) > 0 {
		_ = addHeaderLogo(f, sheet, "N2", assets.NTTLogo, ".png", 0.6, 0.6)
	}

	writeNTTMetadata(f, sheet, in, st)
	writeNTTHeaders(f, sheet, st)
	writeNTTDailyRows(f, sheet, in, st)
	writeNTTProjectSummary(f, sheet, st)
	writeNTTSignatures(f, sheet, in, st)

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
