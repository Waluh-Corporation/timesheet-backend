package services

import (
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"

	"timesheet-backend/assets"
	"timesheet-backend/models"
)

const (
	miiProjectName = "BNI Direct"
	miiProjectID   = "P24015"
	miiDivision    = "Wholesale Digital Delivery"
	miiDepartment  = "Wholesale Channel and Service Delivery"
)

func setMIIColWidths(f *excelize.File, sheet string) {
	widths := map[string]float64{
		"A": 12, "B": 8, "C": 8, "D": 11,
		"E": 8, "F": 8, "G": 13, "H": 8, "I": 8, "J": 12,
		"K": 35, "L": 14, "M": 12, "N": 18, "O": 12, "P": 25, "Q": 25, "R": 15,
	}
	for col, w := range widths {
		_ = f.SetColWidth(sheet, col, col, w)
	}
}

func writeMIIMetadata(f *excelize.File, sheet string, in GenerationInput, st *BuilderStyles) {
	div := in.User.Division
	if div == "" {
		div = miiDivision
	}
	name := in.User.Name
	empID := ResolveEmployeeID(in.User)
	site := in.User.Site
	if site == "" {
		site = "BNI - RDTX"
	}
	periodStr := fmt.Sprintf("%s-%02d", time.Month(in.Month).String()[:3], in.Year%100)

	_ = f.MergeCell(sheet, "A1", "B1")
	_ = f.MergeCell(sheet, "C1", "G1")
	WriteMetaField(f, sheet, "A1", "NAME of PROJECT", "C1", ": "+miiProjectName, st)

	_ = f.MergeCell(sheet, "A2", "B2")
	_ = f.MergeCell(sheet, "C2", "E2")
	WriteMetaField(f, sheet, "A2", "UNIT/DIVISION", "C2", ": "+div, st)

	_ = f.MergeCell(sheet, "A3", "B3")
	_ = f.MergeCell(sheet, "C3", "F3")
	WriteMetaField(f, sheet, "A3", "NAME", "C3", ": "+name, st)

	_ = f.MergeCell(sheet, "A4", "B4")
	WriteMetaField(f, sheet, "A4", "MII ID", "C4", ": "+empID, st)

	_ = f.MergeCell(sheet, "A5", "B5")
	_ = f.MergeCell(sheet, "C5", "F5")
	WriteMetaField(f, sheet, "A5", "SITE", "C5", ": "+site, st)

	_ = f.SetCellStyle(sheet, "G5", "G5", st.HolidayLegendStyle)

	_ = f.MergeCell(sheet, "H5", "I5")
	_ = f.SetCellValue(sheet, "H5", ": Holiday")

	_ = f.MergeCell(sheet, "A6", "B6")
	WriteMetaField(f, sheet, "A6", "PERIODE", "C6", ": "+periodStr, st)

	_ = f.SetCellValue(sheet, "G6", "P = Present; S = Sick;  V = Vacation; BT = Business Trip; PM = Permit; X = Not Working Anymore")
}

func writeMIIHeaders(f *excelize.File, sheet string, st *BuilderStyles) {
	headers := []HeaderColumn{
		{From: "A7", To: "A8", Val: "DATE"},
		{From: "B7", To: "C7", Val: "WORKING HOUR"},
		{From: "D7", To: "D8", Val: "TOTAL HOUR"},
		{From: "E7", To: "J7", Val: "STATUS  ATTENDANCE "},
		{From: "K7", To: "K8", Val: "ACTIVITY / REMARK"},
		{From: "L7", To: "L8", Val: "Project Name"},
		{From: "M7", To: "M8", Val: "Project ID"},
		{From: "N7", To: "N8", Val: "Aplikasi Terdampak"},
		{From: "O7", To: "O8", Val: "AIP Fitur"},
		{From: "P7", To: "P8", Val: "Divisi"},
		{From: "Q7", To: "Q8", Val: "Departement"},
		{From: "R7", To: "R8", Val: "Sub Departement"},
	}
	subHeaders := map[string]string{
		"B8": "START", "C8": "END",
		"E8": "Present", "F8": "Sick ", "G8": "Business Trip",
		"H8": "Permit", "I8": "Vacation", "J8": "Not Working",
	}
	WriteTableHeaders(f, sheet, headers, subHeaders, st)
}

func writeMIIActRow(f *excelize.File, sheet, rs string, row int, act models.DailyActivity, timeStyle int) {
	hasStart, hasEnd := WriteTimeCells(f, sheet, "B"+rs, "C"+rs, act.StartTime, act.EndTime, timeStyle)
	if hasStart && hasEnd {
		_ = f.SetCellFormula(sheet, "D"+rs, fmt.Sprintf("C%s-B%s", rs, rs))
		_ = f.SetCellStyle(sheet, "D"+rs, "D"+rs, timeStyle)
	}

	_ = f.SetCellValue(sheet, "K"+rs, act.Activity)
	_ = f.SetCellValue(sheet, "L"+rs, miiProjectName)
	_ = f.SetCellValue(sheet, "M"+rs, miiProjectID)
	_ = f.SetCellValue(sheet, "N"+rs, NormalizeMIIAppImpacted(act.GetAppImpacted()))
	_ = f.SetCellValue(sheet, "P"+rs, miiDivision)
	_ = f.SetCellValue(sheet, "Q"+rs, miiDepartment)

	h := calculateRowHeight(act.Activity, miiProjectName, miiProjectID, act.GetAppImpacted(), miiDivision, miiDepartment)
	_ = f.SetRowHeight(sheet, row, h)
}

func writeMIIDailyRows(f *excelize.File, sheet string, in GenerationInput, st *BuilderStyles) {
	allCols := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R"}
	WriteBuilderDailyRows(f, sheet, in, 9, allCols, []string{"K", "Q"}, st, func(rs string, row int, dsc DayStyleContext) {
		writeMIIActRow(f, sheet, rs, row, dsc.Activity, dsc.TimeStyle)
	})
}

func writeMIISummaryAndSignatures(f *excelize.File, sheet string, in GenerationInput, st *BuilderStyles) {
	formulas := map[string]string{
		"E": `COUNTIF(E9:E39,"P")`,
		"F": `COUNTIF(F9:F39,"S")`,
		"G": `COUNTIF(G9:G39,"BT")`,
		"H": `COUNTIF(H9:H39,"PM")`,
		"I": `COUNTIF(I9:I39,"V")`,
		"J": `COUNTIF(J9:J39,"x")`,
	}
	WriteColumnFormulas(f, sheet, "40", formulas, st.BoldCenterStyle)

	tlName, dhName := ExtractApprovers(in.Overtimes)

	WriteSignaturesLayout(f, sheet, 42, 3, []SignatureParty{
		{StartCol: "A", EndCol: "C", Title: "TTD PEGAWAI,", Name: in.User.Name, DatePrefix: DatePrefixUpper},
		{StartCol: "D", EndCol: "F", Title: "DIPERIKSA OLEH,", Name: tlName, DatePrefix: DatePrefixUpper},
		{StartCol: "G", EndCol: "J", Title: "DISETUJUI OLEH,", Name: dhName, DatePrefix: DatePrefixUpper},
	}, st)

	_ = f.SetRowHeight(sheet, 42, 20)
	_ = f.SetRowHeight(sheet, 43, 16)
	_ = f.SetRowHeight(sheet, 44, 16)
	_ = f.SetRowHeight(sheet, 45, 18)
	_ = f.SetRowHeight(sheet, 46, 22)
	_ = f.SetRowHeight(sheet, 47, 18)
}

// buildMIIWorkbook generates the MII timesheet Excel document purely from code.
func buildMIIWorkbook(in GenerationInput) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	const sheet = "Sheet1"
	_ = f.SetSheetName(f.GetSheetName(0), sheet)
	userName := ""
	if in.User != nil {
		userName = in.User.Name
	}
	SetWorkbookProperties(f, "Timesheet MII", userName)

	st, err := NewBuilderStyles(f)
	if err != nil {
		return nil, fmt.Errorf("create builder styles: %w", err)
	}

	setMIIColWidths(f, sheet)

	if len(assets.MIILogo) > 0 {
		_ = addHeaderLogoWithCM(f, sheet, "K1", assets.MIILogo, ".png", 3.52, 2.5)
	}

	writeMIIMetadata(f, sheet, in, st)
	writeMIIHeaders(f, sheet, st)
	writeMIIDailyRows(f, sheet, in, st)
	writeMIISummaryAndSignatures(f, sheet, in, st)

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
