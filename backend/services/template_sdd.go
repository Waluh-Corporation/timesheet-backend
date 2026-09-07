package services

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"timesheet-backend/models"
)

// indonesianMonthName returns the Indonesian month name for a 1-indexed month number.
func indonesianMonthName(m int) string {
	months := []string{
		"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
		"Juli", "Agustus", "September", "Oktober", "November", "Desember",
	}
	if m >= 1 && m <= 12 {
		return months[m]
	}
	return ""
}

// generateSDD renders the built-in "SDD Timesheet" template (manual attendance layout):
//   - Header: G2 Name, G3 NPP BNI, G4 Division, G5 Department, G7 Period
//   - Daily entries start at row 12:
//     Col A: No (1..31)
//     Col B: Tanggal (Excel date)
//     Col C: Jam Masuk
//     Col D: Jam Pulang
//     Col E: Total Jam Kerja (=IF(C>D,D+1-C,D-C))
//     Col F..J: Status (F=Hadir, G=Cuti, H=Izin, I=Sakit, J=Lembur) marked with "v"
//     Col K: Project Name ("BNIdirect")
//     Col L: Project Code ("P24015")
//     Col M: AIP Fitur
//     Col N: Kegiatan / Task
func generateSDD(in GenerationInput) ([]byte, error) {
	f, err := excelize.OpenReader(bytes.NewReader(in.Template.FileData))
	if err != nil {
		return nil, fmt.Errorf("open template: %w", err)
	}
	defer f.Close()

	origSheet := f.GetSheetName(0)
	monthName := indonesianMonthName(in.Month)
	sheet := origSheet
	if monthName != "" {
		sheet = monthName
		if origSheet != sheet {
			_ = f.SetSheetName(origSheet, sheet)
		}
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

	// Header metadata (column G).
	if in.User.Name != "" {
		setTyped("G2", in.User.Name)
	}
	if in.User.MiiID != "" {
		setTyped("G3", in.User.MiiID)
	}
	if in.User.Division != "" {
		setTyped("G4", in.User.Division)
	}
	setTyped("G5", "Wholesale Channel and Service Delivery")
	setTyped("G7", fmt.Sprintf("%s %d", indonesianMonthName(in.Month), in.Year))

	byDay := make(map[int]models.DailyActivity, len(in.Activities))
	for _, a := range in.Activities {
		byDay[a.Date.Day()] = a
	}

	allCols := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O"}
	const firstRow = 12
	daysInMonth := GetDaysInMonth(in.Year, in.Month)

	// Unmerge any leftover multi-row merges in daily data rows (rows 12 to 42)
	// from previous sample entries so every day is completely independent.
	if merges, err := f.GetMergeCells(sheet); err == nil {
		for _, m := range merges {
			_, sr, _ := excelize.CellNameToCoordinates(m.GetStartAxis())
			_, er, _ := excelize.CellNameToCoordinates(m.GetEndAxis())
			if sr >= 12 && er >= 12 && er > sr {
				_ = f.UnmergeCell(sheet, m.GetStartAxis(), m.GetEndAxis())
			}
		}
	}

	for day := 1; day <= 31; day++ {
		rs := fmt.Sprintf("%d", firstRow+(day-1))

		if day > daysInMonth {
			for _, col := range allCols {
				setStr(col+rs, "")
			}
			continue
		}

		date := time.Date(in.Year, time.Month(in.Month), day, 0, 0, 0, 0, time.UTC)
		setTyped("A"+rs, day)
		setTyped("B"+rs, date)

		// Clear daily data cells first.
		for _, col := range []string{"C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O"} {
			setStr(col+rs, "")
		}

		isWeekend := date.Weekday() == time.Saturday || date.Weekday() == time.Sunday
		holiday := in.Holidays[day]
		act, hasAct := byDay[day]

		if hasAct {
			hasStart, hasEnd := false, false
			if act.StartTime != "" {
				setTyped("C"+rs, act.StartTime)
				hasStart = true
			}
			if act.EndTime != "" {
				setTyped("D"+rs, act.EndTime)
				hasEnd = true
			}
			if hasStart && hasEnd {
				setFormula("E"+rs, fmt.Sprintf("IF(C%s>D%s,D%s+1-C%s,D%s-C%s)", rs, rs, rs, rs, rs, rs))
			}

			// Mark status: F=Hadir, G=Cuti, H=Izin, I=Sakit, J=Lembur
			st := strings.ToUpper(strings.TrimSpace(act.Status))
			switch st {
			case "P", "H", "HADIR", "PRESENT":
				setStr("F"+rs, "v")
			case "C", "V", "CUTI", "VACATION":
				setStr("G"+rs, "v")
			case "I", "PM", "IZIN", "PERMIT":
				setStr("H"+rs, "v")
			case "S", "SAKIT", "SICK":
				setStr("I"+rs, "v")
			case "L", "LEMBUR":
				setStr("J"+rs, "v")
			default:
				setStr("F"+rs, "v")
			}

			proj := act.ProjectName
			if proj == "" {
				proj = "BNIdirect"
			}
			setStr("K"+rs, proj)

			pCode := act.ProjectID
			if pCode == "" {
				pCode = "P24015"
			}
			setStr("L"+rs, pCode)
			setStr("N"+rs, act.Activity)
		} else if isWeekend || holiday != "" {
			msg := "Libur Akhir Pekan"
			if holiday != "" {
				msg = holiday
			}
			setStr("N"+rs, msg)
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
