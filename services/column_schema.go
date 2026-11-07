package services

// ColumnAlign specifies text alignment within a table cell or PDF column.
type ColumnAlign string

const (
	AlignLeft   ColumnAlign = "left"
	AlignCenter ColumnAlign = "center"
	AlignRight  ColumnAlign = "right"
)

// ColumnDefinition defines a layout column spanning spreadsheet and PDF grid properties.
type ColumnDefinition struct {
	Key       string      `json:"key"`        // Semantic identifier ("date", "time_in", "activity", etc.)
	Title     string      `json:"title"`      // Column header label ("Date", "Time In", etc.)
	ExcelCol  string      `json:"excel_col"`  // Excel column ("A", "B", "C", etc.)
	GridRatio int         `json:"grid_ratio"` // Maroto 12-grid unit width (1..12, sum of row = 12)
	Align     ColumnAlign `json:"align"`      // "left", "center", "right"
}

// StandardTimesheetPDFColumns defines the canonical 12-grid layout for landscape timesheet PDF.
// Sum of GridRatio: 1 + 1 + 1 + 1 + 1 + 1 + 4 + 2 = 12 units.
func StandardTimesheetPDFColumns() []ColumnDefinition {
	return []ColumnDefinition{
		{Key: "date", Title: "Tanggal", ExcelCol: "A", GridRatio: 1, Align: AlignCenter},
		{Key: "day", Title: "Hari", ExcelCol: "A", GridRatio: 1, Align: AlignCenter},
		{Key: "time_in", Title: "Datang", ExcelCol: "B", GridRatio: 1, Align: AlignCenter},
		{Key: "time_out", Title: "Pulang", ExcelCol: "C", GridRatio: 1, Align: AlignCenter},
		{Key: "total_hours", Title: "Total", ExcelCol: "D", GridRatio: 1, Align: AlignCenter},
		{Key: "status", Title: "Status", ExcelCol: "E", GridRatio: 1, Align: AlignCenter},
		{Key: "activity", Title: "Aktivitas / Kegiatan", ExcelCol: "K", GridRatio: 4, Align: AlignLeft},
		{Key: "project", Title: "Proyek / Aplikasi", ExcelCol: "L", GridRatio: 2, Align: AlignLeft},
	}
}

// BeritaAcaraPDFColumns defines the canonical 12-grid layout for portrait Berita Acara PDF table.
// Sum of GridRatio: 1 + 2 + 2 + 2 + 2 + 3 = 12 units.
func BeritaAcaraPDFColumns() []ColumnDefinition {
	return []ColumnDefinition{
		{Key: "no", Title: "No", ExcelCol: "A", GridRatio: 1, Align: AlignCenter},
		{Key: "date", Title: "Tanggal", ExcelCol: "B", GridRatio: 2, Align: AlignCenter},
		{Key: "day", Title: "Hari", ExcelCol: "C", GridRatio: 2, Align: AlignCenter},
		{Key: "time_in", Title: "Datang", ExcelCol: "D", GridRatio: 2, Align: AlignCenter},
		{Key: "time_out", Title: "Pulang", ExcelCol: "E", GridRatio: 2, Align: AlignCenter},
		{Key: "notes", Title: "Keterangan / Justifikasi", ExcelCol: "F", GridRatio: 3, Align: AlignLeft},
	}
}
