package services

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"timesheet-backend/models"
)

// generateAdidata renders the built-in "Adidata Timesheet" template:
//   - Sheet: TIMESHEET (preserving any SPL sheets intact)
//   - Header: C1 Project, C2 Division, C3 Name, C4 NPP, C5 Period
//   - Daily entries start at row 9:
//     Col A: Date
//     Col B: Jam Masuk
//     Col C: Jam Pulang
//     Col D: Total Jam Kerja (=(C-B)*24)
//     Col E..J: Status (E=P, F=S, G=BT, H=PM, I=V, J=X)
//     Col K: Activity / Remark
//     Col L: Nama Project
//     Col M: Project Code
//     Col N: Aplikasi Terdampak
//     Col O: AIP Fitur
func generateAdidata(in GenerationInput) ([]byte, error) {
	f, err := excelize.OpenReader(bytes.NewReader(in.Template.FileData))
	if err != nil {
		return nil, fmt.Errorf("open template: %w", err)
	}
	defer f.Close()

	sheet := "TIMESHEET"
	if sIdx, _ := f.GetSheetIndex(sheet); sIdx == -1 {
		sheet = f.GetSheetName(0)
	}

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

	// Header metadata (column C).
	setTyped("C1", ": PT BANK NEGARA INDONESIA (PERSERO) Tbk")
	div := in.User.Division
	if div == "" {
		div = "BDD / WDL"
	}
	setTyped("C2", ": "+div)
	if in.User.Name != "" {
		setTyped("C3", ": "+in.User.Name)
	}
	if in.User.MiiID != "" {
		setTyped("C4", ": "+in.User.MiiID)
	}
	setTyped("C5", ": "+fmt.Sprintf("%s %d", indonesianMonthName(in.Month), in.Year))

	byDay := make(map[int]models.DailyActivity, len(in.Activities))
	for _, a := range in.Activities {
		byDay[a.Date.Day()] = a
	}
	statusCol := map[string]string{"P": "E", "S": "F", "BT": "G", "PM": "H", "V": "I", "X": "J"}

	allCols := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O"}
	dataCols := []string{"B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O"}
	const firstRow = 9
	daysInMonth := GetDaysInMonth(in.Year, in.Month)

	for day := 1; day <= 31; day++ {
		rs := fmt.Sprintf("%d", firstRow+(day-1))

		if day > daysInMonth {
			for _, col := range allCols {
				setStr(col+rs, "")
			}
			continue
		}

		date := time.Date(in.Year, time.Month(in.Month), day, 0, 0, 0, 0, time.UTC)
		setTyped("A"+rs, date)

		for _, col := range dataCols {
			setStr(col+rs, "")
		}

		isWeekend := date.Weekday() == time.Saturday || date.Weekday() == time.Sunday
		holiday := in.Holidays[day]
		act, hasAct := byDay[day]

		status := ""
		if hasAct {
			status = strings.ToUpper(strings.TrimSpace(act.Status))
			if status == "HADIR" || status == "H" {
				status = "P"
			}

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
			if hasStart && hasEnd {
				setFormula("D"+rs, fmt.Sprintf("(C%s-B%s)*24", rs, rs))
			}

			setStr("K"+rs, act.Activity)
			proj := act.ProjectName
			if proj == "" {
				proj = "PT BANK NEGARA INDONESIA (PERSERO) Tbk"
			}
			setStr("L"+rs, proj)

			pCode := act.ProjectID
			if pCode == "" {
				pCode = "P24015"
			}
			setStr("M"+rs, pCode)
			setStr("N"+rs, NormalizeMIIAppImpacted(act.AppImpacted))
		} else if isWeekend || holiday != "" {
			status = "X"
			if holiday != "" {
				setStr("K"+rs, holiday)
			} else {
				setStr("K"+rs, "Weekend")
			}
		}

		if col, ok := statusCol[status]; ok {
			setStr(col+rs, status)
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
