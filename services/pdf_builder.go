package services

import (
	"bytes"
	"context"
	"fmt"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/image"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/border"
	"github.com/johnfercher/maroto/v2/pkg/consts/extension"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/consts/orientation"
	"github.com/johnfercher/maroto/v2/pkg/consts/pagesize"
	"github.com/johnfercher/maroto/v2/pkg/core"
	"github.com/johnfercher/maroto/v2/pkg/props"
)

func detectImageExtension(data []byte) (extension.Type, bool) {
	if len(data) < 8 {
		return "", false
	}
	if bytes.HasPrefix(data, []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}) {
		return extension.Png, true
	}
	if bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}) {
		return extension.Jpg, true
	}
	return "", false
}

// BuildTimesheetPDF renders an A4 Landscape 12-grid Timesheet PDF using Maroto v2.
func BuildTimesheetPDF(ctx context.Context, model *TimesheetPresentationModel) ([]byte, error) {
	if model == nil {
		return nil, fmt.Errorf("%w: model cannot be nil", ErrPDFGeneration)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	cfg := config.NewBuilder().
		WithPageSize(pagesize.A4).
		WithOrientation(orientation.Horizontal).
		WithLeftMargin(10).
		WithRightMargin(10).
		WithTopMargin(8).
		WithBottomMargin(8).
		Build()

	m := maroto.New(cfg)

	// Document Title & Company
	m.AddRow(7, text.NewCol(12, model.CompanyName, props.Text{
		Style: fontstyle.Bold,
		Size:  12,
		Align: align.Left,
	}))
	m.AddRow(6, text.NewCol(8, "MONTHLY TIMESHEET", props.Text{
		Style: fontstyle.Bold,
		Size:  14,
		Align: align.Left,
	}), text.NewCol(4, model.PeriodDisplay, props.Text{
		Style: fontstyle.Bold,
		Size:  11,
		Align: align.Right,
	}))
	m.AddRow(3, col.New(12))

	// Employee & Project Metadata Header
	m.AddRow(5,
		text.NewCol(6, fmt.Sprintf("Nama Karyawan : %s (%s)", model.EmployeeName, model.EmployeeID), props.Text{Size: 8}),
		text.NewCol(6, fmt.Sprintf("Proyek : %s (%s)", model.ProjectName, model.ProjectID), props.Text{Size: 8}),
	)
	m.AddRow(5,
		text.NewCol(6, fmt.Sprintf("Divisi / Dept   : %s / %s", model.Division, model.Department), props.Text{Size: 8}),
		text.NewCol(6, fmt.Sprintf("Lokasi / Site : %s", model.Site), props.Text{Size: 8}),
	)
	m.AddRow(3, col.New(12))

	// Table Header
	headerCols := StandardTimesheetPDFColumns()
	headerRowCols := make([]core.Col, 0, len(headerCols))
	headerCell := &props.Cell{
		BackgroundColor: &props.Color{Red: 230, Green: 230, Blue: 230},
		BorderType:      border.Full,
		BorderColor:     &props.Color{Red: 180, Green: 180, Blue: 180},
	}

	for _, c := range headerCols {
		headerRowCols = append(headerRowCols, text.NewCol(c.GridRatio, c.Title, props.Text{
			Style: fontstyle.Bold,
			Size:  7.5,
			Align: align.Center,
		}).WithStyle(headerCell))
	}

	m.AddRow(6, headerRowCols...)

	// Table Data Rows
	greyCell := &props.Cell{
		BackgroundColor: &props.Color{Red: 245, Green: 245, Blue: 245},
		BorderType:      border.Full,
		BorderColor:     &props.Color{Red: 210, Green: 210, Blue: 210},
	}
	dataCell := &props.Cell{
		BorderType:  border.Full,
		BorderColor: &props.Color{Red: 220, Green: 220, Blue: 220},
	}

	for _, r := range model.Rows {
		cellStyle := dataCell
		if r.IsNonWorkingRow {
			cellStyle = greyCell
		}

		timeIn := r.StartTime
		timeOut := r.EndTime
		dur := r.DurationFormatted
		status := r.StatusDisplay
		act := r.Activity
		if r.IsNonWorkingRow {
			act = r.Remark
		}

		m.AddRow(4.8,
			text.NewCol(1, r.DateFormatted, props.Text{Size: 6.5, Align: align.Center}).WithStyle(cellStyle),
			text.NewCol(1, r.DayOfWeek, props.Text{Size: 6.5, Align: align.Center}).WithStyle(cellStyle),
			text.NewCol(1, timeIn, props.Text{Size: 6.5, Align: align.Center}).WithStyle(cellStyle),
			text.NewCol(1, timeOut, props.Text{Size: 6.5, Align: align.Center}).WithStyle(cellStyle),
			text.NewCol(1, dur, props.Text{Size: 6.5, Align: align.Center}).WithStyle(cellStyle),
			text.NewCol(1, status, props.Text{Size: 6.5, Align: align.Center}).WithStyle(cellStyle),
			text.NewCol(4, act, props.Text{Size: 6.5, Align: align.Left}).WithStyle(cellStyle),
			text.NewCol(2, r.ProjectName, props.Text{Size: 6.5, Align: align.Left}).WithStyle(cellStyle),
		)
	}

	m.AddRow(3, col.New(12))

	// Summary Row
	sumTxt := fmt.Sprintf("Rekapitulasi: Hari Kerja: %d | Hadir (P): %d | Sakit (S): %d | Perjalanan Dinas (BT): %d | Izin (PM): %d | Cuti (V): %d | Jam Reguler: %.1fh | Lembur: %.1fh",
		model.Summary.TotalWorkingDays,
		model.Summary.TotalPresentDays,
		model.Summary.TotalSickDays,
		model.Summary.TotalBusinessTrip,
		model.Summary.TotalPermitDays,
		model.Summary.TotalLeaveDays,
		model.Summary.TotalWorkingHours,
		model.Summary.TotalOvertimeHours,
	)
	m.AddRow(5, text.NewCol(12, sumTxt, props.Text{
		Style: fontstyle.Bold,
		Size:  7,
		Align: align.Left,
	}))

	m.AddRow(4, col.New(12))

	// Signatures Section (3 Columns: 4 units each)
	m.AddRow(5,
		text.NewCol(4, "Konsultan / Karyawan,", props.Text{Style: fontstyle.Bold, Size: 7.5, Align: align.Center}),
		text.NewCol(4, "Team Leader,", props.Text{Style: fontstyle.Bold, Size: 7.5, Align: align.Center}),
		text.NewCol(4, "Department Head,", props.Text{Style: fontstyle.Bold, Size: 7.5, Align: align.Center}),
	)

	// Image row or spacing
	empExt, hasEmpSig := detectImageExtension(model.Approvers.SignatureBytes)
	qrExt, hasQR := detectImageExtension(model.Approvers.QRCodeBytes)

	empCol := col.New(4)
	if hasEmpSig {
		empCol = image.NewFromBytesCol(4, model.Approvers.SignatureBytes, empExt, props.Rect{Percent: 70, Center: true})
	}

	tlCol := col.New(4)

	dhCol := col.New(4)
	if hasQR {
		dhCol = image.NewFromBytesCol(4, model.Approvers.QRCodeBytes, qrExt, props.Rect{Percent: 70, Center: true})
	}

	m.AddRow(14, empCol, tlCol, dhCol)

	// Signer Names
	m.AddRow(5,
		text.NewCol(4, fmt.Sprintf("( %s )", model.Approvers.EmployeeName), props.Text{Size: 7.5, Align: align.Center}),
		text.NewCol(4, fmt.Sprintf("( %s )", model.Approvers.TeamLeadName), props.Text{Size: 7.5, Align: align.Center}),
		text.NewCol(4, fmt.Sprintf("( %s )", model.Approvers.DepartmentHeadName), props.Text{Size: 7.5, Align: align.Center}),
	)

	doc, err := m.Generate()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPDFGeneration, err)
	}

	pdfBytes := doc.GetBytes()
	if len(pdfBytes) < 5 || !bytes.HasPrefix(pdfBytes, PDFMagicHeader) {
		return nil, fmt.Errorf("%w: output missing %%PDF- magic bytes", ErrPDFGeneration)
	}

	return pdfBytes, nil
}

// BuildBeritaAcaraPDF renders an A4 Portrait 12-grid Berita Acara PDF using Maroto v2.
func BuildBeritaAcaraPDF(ctx context.Context, payload *BeritaAcaraPayload) ([]byte, error) {
	if payload == nil {
		return nil, fmt.Errorf("%w: payload cannot be nil", ErrPDFGeneration)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	cfg := config.NewBuilder().
		WithPageSize(pagesize.A4).
		WithOrientation(orientation.Vertical).
		WithLeftMargin(15).
		WithRightMargin(15).
		WithTopMargin(12).
		WithBottomMargin(12).
		Build()

	m := maroto.New(cfg)

	// Header Title
	m.AddRow(8, text.NewCol(12, "BERITA ACARA KENDALA KEHADIRAN / ABSENSI", props.Text{
		Style: fontstyle.Bold,
		Size:  13,
		Align: align.Center,
	}))
	m.AddRow(4, col.New(12))

	// Preamble narrative
	m.AddRow(5, text.NewCol(12, "Yang bertanda tangan di bawah ini:", props.Text{Size: 9, Style: fontstyle.Normal}))
	m.AddRow(5,
		text.NewCol(3, "Nama", props.Text{Size: 9}),
		text.NewCol(9, ": "+payload.EmployeeName, props.Text{Size: 9, Style: fontstyle.Bold}),
	)
	m.AddRow(5,
		text.NewCol(3, "NPP / ID", props.Text{Size: 9}),
		text.NewCol(9, ": "+payload.NPP, props.Text{Size: 9}),
	)
	m.AddRow(5,
		text.NewCol(3, "Departemen", props.Text{Size: 9}),
		text.NewCol(9, ": "+payload.Department, props.Text{Size: 9}),
	)
	m.AddRow(5,
		text.NewCol(3, "Grup / Unit", props.Text{Size: 9}),
		text.NewCol(9, ": "+payload.GroupName, props.Text{Size: 9}),
	)
	m.AddRow(5,
		text.NewCol(3, "Saksi", props.Text{Size: 9}),
		text.NewCol(9, ": "+payload.WitnessName, props.Text{Size: 9}),
	)
	m.AddRow(3, col.New(12))
	m.AddRow(8, text.NewCol(12, "Menyatakan bahwa telah terjadi kendala pencatatan kehadiran pada sistem Timesheet dengan rincian hari dan keterangan justifikasi sebagai berikut:", props.Text{
		Size:  8.5,
		Align: align.Left,
	}))
	m.AddRow(3, col.New(12))

	// Incidents Table Header
	headerCols := BeritaAcaraPDFColumns()
	headerCell := &props.Cell{
		BackgroundColor: &props.Color{Red: 235, Green: 235, Blue: 235},
		BorderType:      border.Full,
		BorderColor:     &props.Color{Red: 180, Green: 180, Blue: 180},
	}

	headerNodes := make([]core.Col, 0, len(headerCols))
	for _, c := range headerCols {
		headerNodes = append(headerNodes, text.NewCol(c.GridRatio, c.Title, props.Text{
			Style: fontstyle.Bold,
			Size:  8,
			Align: align.Center,
		}).WithStyle(headerCell))
	}
	m.AddRow(6, headerNodes...)

	// Incident Rows
	dataCell := &props.Cell{
		BorderType:  border.Full,
		BorderColor: &props.Color{Red: 210, Green: 210, Blue: 210},
	}

	if len(payload.Items) == 0 {
		m.AddRow(6, text.NewCol(12, "Tidak ada data kendala absensi pada periode ini.", props.Text{
			Size:  8,
			Style: fontstyle.Normal,
			Align: align.Center,
		}).WithStyle(dataCell))
	} else {
		for idx, it := range payload.Items {
			dateStr := ""
			if !it.Date.IsZero() {
				dateStr = it.Date.Format("02/01/2006")
			}
			dayName := it.Day
			if dayName == "" && !it.Date.IsZero() {
				dayName = IndonesianDayName(it.Date.Weekday())
			}

			m.AddRow(6,
				text.NewCol(1, fmt.Sprintf("%d", idx+1), props.Text{Size: 7.5, Align: align.Center}).WithStyle(dataCell),
				text.NewCol(2, dateStr, props.Text{Size: 7.5, Align: align.Center}).WithStyle(dataCell),
				text.NewCol(2, dayName, props.Text{Size: 7.5, Align: align.Center}).WithStyle(dataCell),
				text.NewCol(2, it.StartTime, props.Text{Size: 7.5, Align: align.Center}).WithStyle(dataCell),
				text.NewCol(2, it.EndTime, props.Text{Size: 7.5, Align: align.Center}).WithStyle(dataCell),
				text.NewCol(3, it.Notes, props.Text{Size: 7.5, Align: align.Left}).WithStyle(dataCell),
			)
		}
	}

	m.AddRow(6, col.New(12))

	// Closing text
	m.AddRow(7, text.NewCol(12, "Demikian Berita Acara ini dibuat dengan sebenarnya untuk dapat dipergunakan sebagaimana mestinya.", props.Text{
		Size:  8.5,
		Align: align.Left,
	}))
	m.AddRow(6, col.New(12))

	// Dual Signatures: Witness on Left (col 6), Employee on Right (col 6)
	m.AddRow(5,
		text.NewCol(6, "Mengetahui / Saksi,", props.Text{Style: fontstyle.Bold, Size: 8.5, Align: align.Center}),
		text.NewCol(6, "Pemohon,", props.Text{Style: fontstyle.Bold, Size: 8.5, Align: align.Center}),
	)

	witExt, hasWitSig := detectImageExtension(payload.WitnessSignatureBytes)
	empExt, hasEmpSig := detectImageExtension(payload.EmployeeSignatureBytes)

	witCol := col.New(6)
	if hasWitSig {
		witCol = image.NewFromBytesCol(6, payload.WitnessSignatureBytes, witExt, props.Rect{Percent: 65, Center: true})
	}

	empCol := col.New(6)
	if hasEmpSig {
		empCol = image.NewFromBytesCol(6, payload.EmployeeSignatureBytes, empExt, props.Rect{Percent: 65, Center: true})
	}

	m.AddRow(18, witCol, empCol)

	m.AddRow(5,
		text.NewCol(6, fmt.Sprintf("( %s )", payload.WitnessName), props.Text{Size: 8.5, Align: align.Center}),
		text.NewCol(6, fmt.Sprintf("( %s )", payload.EmployeeName), props.Text{Size: 8.5, Align: align.Center}),
	)

	m.AddRow(6, col.New(12))

	// Footer QR Code & Verification Label
	if qrExt, hasQR := detectImageExtension(payload.QRCodeBytes); hasQR {
		m.AddRow(16, image.NewFromBytesCol(12, payload.QRCodeBytes, qrExt, props.Rect{Percent: 40, Center: true}))
		m.AddRow(4, text.NewCol(12, "Dokumen ini telah diverifikasi secara elektronik via Timesheet Platform.", props.Text{
			Size:  7,
			Align: align.Center,
			Style: fontstyle.Normal,
		}))
	}

	doc, err := m.Generate()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPDFGeneration, err)
	}

	pdfBytes := doc.GetBytes()
	if len(pdfBytes) < 5 || !bytes.HasPrefix(pdfBytes, PDFMagicHeader) {
		return nil, fmt.Errorf("%w: output missing %%PDF- magic bytes", ErrPDFGeneration)
	}

	return pdfBytes, nil
}
