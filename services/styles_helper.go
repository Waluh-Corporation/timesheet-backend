package services

import (
	"github.com/xuri/excelize/v2"
)

// BuilderStyles bundles pre-created cell style IDs for programmatic sheet generation.
type BuilderStyles struct {
	HeaderStyle         int
	HeaderGreyStyle     int
	DataCenterStyle     int
	DataCenterWrapStyle int
	DataLeftStyle       int
	DateStyle           int
	TimeStyle           int
	DecimalStyle        int
	BoldCenterStyle     int
	BoldLeftStyle       int
	MetaLabelStyle      int
	MetaValueStyle      int
	HolidayLegendStyle  int
	GreyDateStyle       int
	GreyCenterStyle     int
	GreyCenterWrapStyle int
	GreyTimeStyle       int
	GreyDecimalStyle    int
}

// NewBuilderStyles registers reusable Excel styles on the workbook.
func NewBuilderStyles(f *excelize.File) (*BuilderStyles, error) {
	thinBorder := []excelize.Border{
		{Type: "top", Color: "000000", Style: 1},
		{Type: "bottom", Color: "000000", Style: 1},
		{Type: "left", Color: "000000", Style: 1},
		{Type: "right", Color: "000000", Style: 1},
	}

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10, Family: "Calibri"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#D9E1F2"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border:    thinBorder,
	})
	if err != nil {
		return nil, err
	}

	headerGreyStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10, Family: "Calibri"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#E7E6E6"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border:    thinBorder,
	})

	dataCenterStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 9, Family: "Calibri"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    thinBorder,
	})

	dataCenterWrapStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 9, Family: "Calibri"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border:    thinBorder,
	})

	dataLeftStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 9, Family: "Calibri"},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center", WrapText: true},
		Border:    thinBorder,
	})

	dateStyle, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Size: 9, Family: "Calibri"},
		Alignment:    &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:       thinBorder,
		CustomNumFmt: strPtr("d-mmm-yy"),
	})

	timeStyle, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Size: 9, Family: "Calibri"},
		Alignment:    &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:       thinBorder,
		CustomNumFmt: strPtr("hh:mm"),
	})

	decimalStyle, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Size: 9, Family: "Calibri"},
		Alignment:    &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:       thinBorder,
		CustomNumFmt: strPtr("0.00"),
	})

	boldCenterStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 9, Family: "Calibri"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    thinBorder,
	})

	boldLeftStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 9, Family: "Calibri"},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
		Border:    thinBorder,
	})

	metaLabelStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10, Family: "Calibri"},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
	})

	metaValueStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10, Family: "Calibri"},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center", WrapText: false},
	})

	const colorGrey = "#AEAAAA"

	holidayLegendStyle, _ := f.NewStyle(&excelize.Style{
		Fill:   excelize.Fill{Type: "pattern", Color: []string{colorGrey}, Pattern: 1},
		Border: thinBorder,
	})

	greyDateStyle, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Size: 9, Family: "Calibri"},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{colorGrey}, Pattern: 1},
		Alignment:    &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:       thinBorder,
		CustomNumFmt: strPtr("d-mmm-yy"),
	})

	greyCenterStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 9, Family: "Calibri"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{colorGrey}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    thinBorder,
	})

	greyCenterWrapStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 9, Family: "Calibri"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{colorGrey}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border:    thinBorder,
	})

	greyTimeStyle, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Size: 9, Family: "Calibri"},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{colorGrey}, Pattern: 1},
		Alignment:    &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:       thinBorder,
		CustomNumFmt: strPtr("hh:mm"),
	})

	greyDecimalStyle, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Size: 9, Family: "Calibri"},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{colorGrey}, Pattern: 1},
		Alignment:    &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:       thinBorder,
		CustomNumFmt: strPtr("0.00"),
	})

	return &BuilderStyles{
		HeaderStyle:         headerStyle,
		HeaderGreyStyle:     headerGreyStyle,
		DataCenterStyle:     dataCenterStyle,
		DataCenterWrapStyle: dataCenterWrapStyle,
		DataLeftStyle:       dataLeftStyle,
		DateStyle:           dateStyle,
		TimeStyle:           timeStyle,
		DecimalStyle:        decimalStyle,
		BoldCenterStyle:     boldCenterStyle,
		BoldLeftStyle:       boldLeftStyle,
		MetaLabelStyle:      metaLabelStyle,
		MetaValueStyle:      metaValueStyle,
		HolidayLegendStyle:  holidayLegendStyle,
		GreyDateStyle:       greyDateStyle,
		GreyCenterStyle:     greyCenterStyle,
		GreyCenterWrapStyle: greyCenterWrapStyle,
		GreyTimeStyle:       greyTimeStyle,
		GreyDecimalStyle:    greyDecimalStyle,
	}, nil
}

func strPtr(s string) *string {
	return &s
}

// styleMergedRange merges topCell to botCell and applies styleID to all cells in the rectangular range.
func styleMergedRange(f *excelize.File, sheet, topCell, botCell string, styleID int) {
	_ = f.MergeCell(sheet, topCell, botCell)
	_ = f.SetCellStyle(sheet, topCell, botCell, styleID)
}
