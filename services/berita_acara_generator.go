package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/ZeroHawkeye/wordZero/pkg/document"

	"timesheet-backend/models"
)

var indonesianDayNames = map[time.Weekday]string{
	time.Sunday:    "Minggu",
	time.Monday:    "Senin",
	time.Tuesday:   "Selasa",
	time.Wednesday: "Rabu",
	time.Thursday:  "Kamis",
	time.Friday:    "Jumat",
	time.Saturday:  "Sabtu",
}

// IndonesianDayName returns the Indonesian day name for a given date.
func IndonesianDayName(t time.Time) string {
	return indonesianDayNames[t.Weekday()]
}

// BeritaAcaraInput contains everything needed to render a Berita Acara document.
type BeritaAcaraInput struct {
	CompanyCode  string
	User         *models.User
	Month        int
	Year         int
	BeritaAcaras []models.BeritaAcara
	Approvers    []models.Approver
}

// ResolveBeritaAcaraApprovers extracts team leader and department head from berita acara entries or master approvers.
func ResolveBeritaAcaraApprovers(items []models.BeritaAcara, masterApprovers []models.Approver) (tlName string, dhName string) {
	for _, it := range items {
		if it.TeamLeader != nil && it.TeamLeader.Name != "" && tlName == "" {
			tlName = it.TeamLeader.Name
		}
		if it.DepartmentHead != nil && it.DepartmentHead.Name != "" && dhName == "" {
			dhName = it.DepartmentHead.Name
		}
	}
	for _, appr := range masterApprovers {
		if tlName == "" && appr.RoleType == models.ApproverRoleTeamLeader && appr.IsActive {
			tlName = appr.Name
		}
		if dhName == "" && appr.RoleType == models.ApproverRoleDepartmentHead && appr.IsActive {
			dhName = appr.Name
		}
	}
	return tlName, dhName
}

// GenerateBeritaAcaraDocx programmatically constructs the Berita Acara Word document using WordZero.
func GenerateBeritaAcaraDocx(in BeritaAcaraInput) ([]byte, error) {
	doc := document.New()
	_ = doc.SetPageMargins(20, 20, 20, 20)

	tlName, dhName := ResolveBeritaAcaraApprovers(in.BeritaAcaras, in.Approvers)
	userName := ""
	bniID := ""
	dept := ""
	group := ""
	div := ""

	if in.User != nil {
		userName = in.User.Name
		if in.User.BniID != "" {
			bniID = in.User.BniID
		} else {
			bniID = in.User.EmployeeID
		}
		if in.User.DepartmentRel != nil && in.User.DepartmentRel.Name != "" {
			dept = in.User.DepartmentRel.Name
		} else {
			dept = in.User.Department
		}
		group = in.User.GroupName
		div = in.User.Division
	}

	company := strings.ToLower(strings.TrimSpace(in.CompanyCode))

	// 1. Build Header Paragraphs
	switch company {
	case "sdd":
		doc.AddParagraph("PT. BANK NEGARA INDONESIA (Persero) Tbk")
		divText := div
		if divText == "" {
			divText = "WDL"
		}
		doc.AddParagraph("DIVISI : " + divText)
		doc.AddParagraph("")
		pTitle := doc.AddFormattedParagraph("SURAT PERNYATAAN KEHADIRAN", &document.TextFormat{
			Bold:     true,
			FontSize: 12,
		})
		pTitle.SetAlignment(document.AlignCenter)
		doc.AddParagraph("")
		doc.AddParagraph("Yang Bertanda tangan di bawah ini menerangkan bahwa :")
		doc.AddParagraph("Nama: " + userName)
		doc.AddParagraph("NPP BNI: " + bniID)
		doc.AddParagraph("Departemen: " + dept)
		doc.AddParagraph("Kelompok: " + group)
		doc.AddParagraph("")

	case "adidata":
		pTitle := doc.AddFormattedParagraph("BERITA ACARA", &document.TextFormat{
			Bold:     true,
			FontSize: 12,
		})
		pTitle.SetAlignment(document.AlignCenter)
		doc.AddParagraph("")
		doc.AddParagraph("Yang bertandatangan di bawah ini menerangkan bahwa:")
		doc.AddParagraph("Nama: " + userName)
		doc.AddParagraph("NPP BNI: " + bniID)
		doc.AddParagraph("Departement: " + dept)
		doc.AddParagraph("Kelompok: " + group)
		doc.AddParagraph("")

	default: // mii & others
		pTitle := doc.AddFormattedParagraph("BERITA ACARA", &document.TextFormat{
			Bold:     true,
			FontSize: 12,
		})
		pTitle.SetAlignment(document.AlignCenter)
		doc.AddParagraph("")
		doc.AddParagraph("Yang bertandatangan di bawah ini menerangkan bahwa:")
		doc.AddParagraph("Nama: " + userName)
		doc.AddParagraph("NPP BNI: " + bniID)
		doc.AddParagraph("Departement: " + dept)
		doc.AddParagraph("Kelompok: " + group)
		doc.AddParagraph("")
	}

	// 2. Build Table Data
	var headers []string
	if company == "sdd" {
		headers = []string{"Tanggal", "Hari", "Jam Datang", "Jam Pulang", "Keterangan"}
	} else {
		headers = []string{"TANGGAL", "HARI", "JAM DATANG", "JAM PULANG", "KETERANGAN"}
	}

	tableData := [][]string{headers}
	if len(in.BeritaAcaras) > 0 {
		for _, it := range in.BeritaAcaras {
			dayStr := it.Day
			if dayStr == "" {
				dayStr = IndonesianDayName(it.Date)
			}
			tableData = append(tableData, []string{
				it.Date.Format("02/01/2006"),
				dayStr,
				it.StartTime,
				it.EndTime,
				it.Keterangan,
			})
		}
	} else {
		tableData = append(tableData, []string{"-", "-", "-", "-", "-"})
	}

	tableCfg := &document.TableConfig{
		Rows: len(tableData),
		Cols: 5,
		Data: tableData,
	}

	tbl, err := doc.AddTable(tableCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to add table to docx: %w", err)
	}

	border := &document.BorderConfig{
		Style: document.BorderStyleSingle,
		Width: 4,
		Color: "auto",
	}
	_ = tbl.SetTableBorders(&document.TableBorderConfig{
		Top:     border,
		Left:    border,
		Bottom:  border,
		Right:   border,
		InsideH: border,
		InsideV: border,
	})
	_ = tbl.SetTableAlignment(document.TableAlignCenter)

	// 3. Build Footer / Signatures
	doc.AddParagraph("")
	switch company {
	case "sdd":
		doc.AddParagraph("Sehubungan dengan hal tersebut, mohon dapat diperhitungkan aktivitas saya pada hari itu sebagai kehadiran.")
		doc.AddParagraph("Demikian harap maklum. Atas perhatiannya saya ucapkan terima kasih.")
		doc.AddParagraph("")
		doc.AddParagraph("Saksi:                                                              Hormat Saya,")
		doc.AddParagraph("")
		doc.AddParagraph("")
		tlLabel := tlName
		if tlLabel == "" {
			tlLabel = "  Nama Team Leader  "
		}
		userLabel := userName
		if userLabel == "" {
			userLabel = "  Nama Karyawan  "
		}
		doc.AddParagraph(fmt.Sprintf("(  %-25s )             ( %-25s )", tlLabel, userLabel))

	case "adidata":
		doc.AddParagraph("Hormat saya,                                                    Mengetahui,")
		doc.AddParagraph("")
		doc.AddParagraph("")
		userLabel := userName
		if userLabel == "" {
			userLabel = "Nama Karyawan"
		}
		tlLabel := tlName
		if tlLabel == "" {
			tlLabel = "Nama Lead"
		}
		doc.AddParagraph(fmt.Sprintf("(%-30s)                   (%-30s)", userLabel, tlLabel))
		doc.AddParagraph("")
		doc.AddParagraph("Menyetujui")
		doc.AddParagraph("")
		doc.AddParagraph("")
		dhLabel := dhName
		if dhLabel == "" {
			dhLabel = "____________________"
		}
		doc.AddParagraph("(" + dhLabel + ")")
		doc.AddParagraph("DEPARTMENT HEAD")

	default: // mii & others
		doc.AddParagraph("Saksi                                                                Hormat Saya,")
		doc.AddParagraph("")
		doc.AddParagraph("")
		tlLabel := tlName
		if tlLabel == "" {
			tlLabel = "Nama Karyawan"
		}
		userLabel := userName
		if userLabel == "" {
			userLabel = "Nama Karyawan"
		}
		doc.AddParagraph(fmt.Sprintf("(%-30s)                   (%-30s)", tlLabel, userLabel))
		doc.AddParagraph("")
		doc.AddParagraph("Menyetujui")
		doc.AddParagraph("")
		doc.AddParagraph("")
		apprLabel := dhName
		if apprLabel == "" {
			apprLabel = tlName
		}
		if apprLabel == "" {
			apprLabel = "Nama Team Lead"
		}
		doc.AddParagraph("(" + apprLabel + ")")
		doc.AddParagraph("Team Lead")
	}

	return doc.ToBytes()
}
