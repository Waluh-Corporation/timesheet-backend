package service

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/mailer"
	"timesheet-backend/models"
	"timesheet-backend/services"
	"timesheet-backend/storage"
)

// TimesheetService defines business operations for generating timesheets and managing overtime records.
type TimesheetService interface {
	GenerateWorkbook(ctx context.Context, userID uint, month int, year int) ([]byte, string, error)
	GenerateBeritaAcara(ctx context.Context, userID uint, month int, year int) ([]byte, string, error)
	UpsertOvertime(ctx context.Context, userID uint, req *request.OvertimeRequest) error
	ListMonthlyOvertimes(ctx context.Context, userID uint, month int, year int) ([]response.OvertimeResponse, error)
	DeleteOvertime(ctx context.Context, id uint, userID uint) error
}

// TimesheetPDFGenerator defines optional methods for generating timesheet and Berita Acara PDF documents.
type TimesheetPDFGenerator interface {
	GenerateTimesheetPDF(ctx context.Context, userID uint, month int, year int) ([]byte, string, error)
	GenerateBeritaAcaraPDF(ctx context.Context, userID uint, month int, year int) ([]byte, string, error)
}

// TimesheetBundler defines operations for bundling timesheet artifacts into single files or archives.
type TimesheetBundler interface {
	PackageTimesheetBundle(ctx context.Context, userID uint, month int, year int, includePDF bool) ([]byte, string, string, error)
}

type timesheetService struct {
	userRepo     repository.UserAccountRepository
	activityRepo repository.ActivityRepository
	overtimeRepo repository.OvertimeRepository
	masterRepo   repository.MasterRepository
	subRepo      repository.SubmissionRepository
	storageSvc   storage.StorageService
	mailer       *mailer.Mailer
	baseURL      string
	nowFunc      func() time.Time
}

// NewTimesheetService constructs a TimesheetService implementation.
func NewTimesheetService(
	userRepo repository.UserAccountRepository,
	activityRepo repository.ActivityRepository,
	overtimeRepo repository.OvertimeRepository,
	masterRepo repository.MasterRepository,
	m *mailer.Mailer,
) TimesheetService {
	return &timesheetService{
		userRepo:     userRepo,
		activityRepo: activityRepo,
		overtimeRepo: overtimeRepo,
		masterRepo:   masterRepo,
		mailer:       m,
		nowFunc:      time.Now,
	}
}

// SetNowFunc overrides the time provider for testing purposes.
func (s *timesheetService) SetNowFunc(fn func() time.Time) {
	s.nowFunc = fn
}

func (s *timesheetService) SetSignatureDependencies(subRepo repository.SubmissionRepository, storageSvc storage.StorageService, baseURL string) {
	s.subRepo = subRepo
	s.storageSvc = storageSvc
	s.baseURL = baseURL
}

func (s *timesheetService) fetchSignatureBytes(ctx context.Context, key string) ([]byte, error) {
	if s.storageSvc == nil || key == "" {
		return nil, nil
	}
	return s.storageSvc.Download(ctx, key)
}

func sanitizeFilename(s string) string {
	var out []rune
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			out = append(out, r)
		}
	}
	return string(out)
}

func (s *timesheetService) buildGenerationInput(ctx context.Context, userID uint, month int, year int) (*services.GenerationInput, error) {
	loc, err := s.validateTimesheetPeriod(ctx, month, year)
	if err != nil {
		return nil, err
	}

	user, err := s.userRepo.FindByIDWithDetails(ctx, userID)
	if err != nil || user == nil {
		return nil, domain.NewUserError(domain.ErrNotFound, "User not found")
	}

	companyCode, err := s.resolveCompanyCode(ctx, user)
	if err != nil {
		return nil, err
	}

	filter := repository.ActivityFilter{
		Month:     &month,
		Year:      &year,
		SortOrder: "asc",
		IsAll:     true,
	}
	activities, _, err := s.activityRepo.ListActiveByUser(ctx, user.ID, filter)
	if err != nil {
		return nil, err
	}
	if len(activities) == 0 {
		return nil, domain.NewUserError(domain.ErrInvalidInput, "No activities recorded for this period")
	}

	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, 0)
	overtimes, err := s.overtimeRepo.FindByUserAndMonth(ctx, user.ID, start, end)
	if err != nil {
		return nil, err
	}

	var sigBytes, qrBytes []byte
	isSigned := false
	if s.subRepo != nil {
		sub, _ := s.subRepo.FindTimesheetSubmission(ctx, user.ID, month, year)
		if sub != nil && (sub.Status == models.SubmissionStatusSigned || sub.Status == models.SubmissionStatusSubmitted) {
			isSigned = true
			if user.SignatureFileKey != "" && s.storageSvc != nil {
				sigBytes, _ = s.fetchSignatureBytes(ctx, user.SignatureFileKey)
			}
			baseURL := strings.TrimRight(s.baseURL, "/")
			if baseURL == "" {
				baseURL = "https://timesheet.local"
			}
			qrURL := fmt.Sprintf("%s/verify/doc/%s", baseURL, sub.VerificationToken)
			qrBytes, _ = services.GenerateQRCodePNG(qrURL, 120)
		}
	}

	return &services.GenerationInput{
		CompanyCode:    companyCode,
		User:           user,
		Month:          month,
		Year:           year,
		Activities:     activities,
		Overtimes:      overtimes,
		Approvers:      s.fetchActiveApprovers(ctx),
		Holidays:       s.fetchHolidaysMap(ctx, year, month),
		SignatureBytes: sigBytes,
		QRCodeBytes:    qrBytes,
		IsSigned:       isSigned,
	}, nil
}

func (s *timesheetService) GenerateWorkbook(ctx context.Context, userID uint, month int, year int) ([]byte, string, error) {
	input, err := s.buildGenerationInput(ctx, userID, month, year)
	if err != nil {
		return nil, "", err
	}

	out, err := services.GenerateFromTemplate(*input)
	if err != nil {
		return nil, "", err
	}

	filename := fmt.Sprintf("Timesheet_%s_%02d_%04d.xlsx", sanitizeFilename(input.User.Username), month, year)
	return out, filename, nil
}

func (s *timesheetService) GenerateTimesheetPDF(ctx context.Context, userID uint, month int, year int) ([]byte, string, error) {
	input, err := s.buildGenerationInput(ctx, userID, month, year)
	if err != nil {
		return nil, "", err
	}

	model, err := services.CalculateTimesheetModel(input)
	if err != nil {
		return nil, "", fmt.Errorf("calculate timesheet model: %w", err)
	}

	pdfBytes, err := services.BuildTimesheetPDF(ctx, model)
	if err != nil {
		return nil, "", fmt.Errorf("build timesheet pdf: %w", err)
	}

	filename := fmt.Sprintf("Timesheet_%s_%02d_%04d.pdf", sanitizeFilename(input.User.Username), month, year)
	return pdfBytes, filename, nil
}

func filterBeritaAcaraActivities(activities []models.DailyActivity) []models.DailyActivity {
	var ba []models.DailyActivity
	for _, a := range activities {
		if a.IsBeritaAcara {
			ba = append(ba, a)
		}
	}
	return ba
}

func (s *timesheetService) resolveBeritaAcaraSignatures(ctx context.Context, user *models.User, month, year int) (bool, string, []byte, []byte, []byte) {
	if s.subRepo == nil {
		return false, "", nil, nil, nil
	}
	sub, _ := s.subRepo.FindBeritaAcaraSubmission(ctx, user.ID, month, year)
	if sub == nil {
		return false, "", nil, nil, nil
	}
	witnessName := s.getSubmissionWitnessName(ctx, sub)
	if sub.Status != models.SubmissionStatusSigned {
		return false, witnessName, nil, nil, nil
	}

	empSig, _ := s.fetchUserSignature(ctx, user.SignatureFileKey)
	witSig := s.resolveWitnessSignatureBytes(ctx, sub)
	qrBytes := s.generateVerificationQR(sub.VerificationToken)
	return true, witnessName, empSig, witSig, qrBytes
}

func (s *timesheetService) getSubmissionWitnessName(ctx context.Context, sub *models.BeritaAcaraSubmission) string {
	if sub.Witness != nil && sub.Witness.Name != "" {
		return sub.Witness.Name
	}
	if sub.WitnessID != 0 && s.userRepo != nil {
		if wUser, err := s.userRepo.FindByID(ctx, sub.WitnessID); err == nil && wUser != nil {
			return wUser.Name
		}
	}
	return ""
}

func (s *timesheetService) fetchUserSignature(ctx context.Context, key string) ([]byte, error) {
	if key == "" || s.storageSvc == nil {
		return nil, nil
	}
	return s.fetchSignatureBytes(ctx, key)
}

func (s *timesheetService) resolveWitnessSignatureBytes(ctx context.Context, sub *models.BeritaAcaraSubmission) []byte {
	if sub.Witness != nil && sub.Witness.SignatureFileKey != "" {
		b, _ := s.fetchUserSignature(ctx, sub.Witness.SignatureFileKey)
		return b
	}
	if sub.WitnessID != 0 && s.userRepo != nil {
		if wUser, err := s.userRepo.FindByID(ctx, sub.WitnessID); err == nil && wUser != nil && wUser.SignatureFileKey != "" {
			b, _ := s.fetchUserSignature(ctx, wUser.SignatureFileKey)
			return b
		}
	}
	return nil
}

func (s *timesheetService) generateVerificationQR(token string) []byte {
	baseURL := strings.TrimRight(s.baseURL, "/")
	if baseURL == "" {
		baseURL = "https://timesheet.local"
	}
	qrURL := fmt.Sprintf("%s/verify/doc/%s", baseURL, token)
	qrBytes, _ := services.GenerateQRCodePNG(qrURL, 120)
	return qrBytes
}

func (s *timesheetService) buildBeritaAcaraPayload(ctx context.Context, userID uint, month int, year int) (*services.BeritaAcaraPayload, *models.User, error) {
	loc, err := s.validateTimesheetPeriod(ctx, month, year)
	if err != nil {
		return nil, nil, err
	}

	user, err := s.userRepo.FindByIDWithDetails(ctx, userID)
	if err != nil || user == nil {
		return nil, nil, domain.NewUserError(domain.ErrNotFound, "User not found")
	}

	filter := repository.ActivityFilter{
		Month:     &month,
		Year:      &year,
		SortOrder: "asc",
		IsAll:     true,
	}
	activities, _, err := s.activityRepo.ListActiveByUser(ctx, user.ID, filter)
	if err != nil {
		return nil, nil, err
	}

	baActivities := filterBeritaAcaraActivities(activities)
	if len(baActivities) == 0 {
		return nil, user, nil
	}

	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, 0)
	overtimes, _ := s.overtimeRepo.FindByUserAndMonth(ctx, user.ID, start, end)

	teamLeadName, _ := services.ResolveApprovers(services.GenerationInput{
		Overtimes: overtimes,
		Approvers: s.fetchActiveApprovers(ctx),
	})

	deptName := user.Department
	if user.DepartmentRel != nil && user.DepartmentRel.Name != "" {
		deptName = user.DepartmentRel.Name
	}
	empName := user.Name
	if empName == "" {
		empName = user.Username
	}

	isSigned, witnessName, empSigBytes, witSigBytes, qrBytes := s.resolveBeritaAcaraSignatures(ctx, user, month, year)
	if witnessName == "" {
		witnessName = s.resolveWitnessName(ctx, user.ID, month, year, baActivities)
	}

	payload := &services.BeritaAcaraPayload{
		EmployeeName:           empName,
		NPP:                    user.BniID,
		Department:             deptName,
		GroupName:              user.GroupName,
		WitnessName:            witnessName,
		TeamLeadName:           teamLeadName,
		Items:                  buildBeritaAcaraItems(baActivities),
		EmployeeSignatureBytes: empSigBytes,
		WitnessSignatureBytes:  witSigBytes,
		QRCodeBytes:            qrBytes,
		IsSigned:               isSigned,
	}
	return payload, user, nil
}

func (s *timesheetService) GenerateBeritaAcara(ctx context.Context, userID uint, month int, year int) ([]byte, string, error) {
	payload, user, err := s.buildBeritaAcaraPayload(ctx, userID, month, year)
	if err != nil {
		return nil, "", err
	}
	if payload == nil {
		return nil, "", nil
	}

	out, err := services.GenerateBeritaAcaraDocx(*payload)
	if err != nil {
		return nil, "", err
	}

	filename := fmt.Sprintf("Berita_Acara_%s_%02d_%04d.docx", sanitizeFilename(user.Username), month, year)
	return out, filename, nil
}

func (s *timesheetService) GenerateBeritaAcaraPDF(ctx context.Context, userID uint, month int, year int) ([]byte, string, error) {
	payload, user, err := s.buildBeritaAcaraPayload(ctx, userID, month, year)
	if err != nil {
		return nil, "", err
	}
	if payload == nil {
		return nil, "", nil
	}

	pdfBytes, err := services.BuildBeritaAcaraPDF(ctx, payload)
	if err != nil {
		return nil, "", fmt.Errorf("build berita acara pdf: %w", err)
	}

	filename := fmt.Sprintf("Berita_Acara_%s_%02d_%04d.pdf", sanitizeFilename(user.Username), month, year)
	return pdfBytes, filename, nil
}

func (s *timesheetService) PackageTimesheetBundle(ctx context.Context, userID uint, month int, year int, includePDF bool) ([]byte, string, string, error) {
	genInput, err := s.buildGenerationInput(ctx, userID, month, year)
	if err != nil {
		return nil, "", "", err
	}

	out, err := services.GenerateFromTemplate(*genInput)
	if err != nil {
		return nil, "", "", err
	}
	xlsxFilename := fmt.Sprintf("Timesheet_%s_%02d_%04d.xlsx", sanitizeFilename(genInput.User.Username), month, year)
	baseName := strings.TrimSuffix(xlsxFilename, filepath.Ext(xlsxFilename))

	baPayload, baUser, baErr := s.buildBeritaAcaraPayload(ctx, userID, month, year)
	if baErr != nil {
		return nil, "", "", baErr
	}
	var baDocxBytes []byte
	var baDocxFilename string
	if baPayload != nil {
		baDocxBytes, err = services.GenerateBeritaAcaraDocx(*baPayload)
		if err != nil {
			return nil, "", "", fmt.Errorf("failed to generate berita acara docx: %w", err)
		}
		baDocxFilename = fmt.Sprintf("Berita_Acara_%s_%02d_%04d.docx", sanitizeFilename(baUser.Username), month, year)
	}

	if !includePDF {
		if baPayload == nil {
			return out, xlsxFilename, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil
		}
		zipBytes, zErr := createZipArchive(map[string][]byte{
			xlsxFilename:   out,
			baDocxFilename: baDocxBytes,
		})
		if zErr != nil {
			return nil, "", "", zErr
		}
		return zipBytes, fmt.Sprintf("%s_and_Berita_Acara.zip", baseName), "application/zip", nil
	}

	// includePDF == true: render Timesheet PDF via pure-Go Maroto v2
	model, err := services.CalculateTimesheetModel(genInput)
	if err != nil {
		return nil, "", "", fmt.Errorf("calculate timesheet model: %w", err)
	}
	tsPdfBytes, err := services.BuildTimesheetPDF(ctx, model)
	if err != nil {
		return nil, "", "", fmt.Errorf("build timesheet pdf: %w", err)
	}
	tsPdfFilename := fmt.Sprintf("%s.pdf", baseName)

	filesToZip := map[string][]byte{
		xlsxFilename:  out,
		tsPdfFilename: tsPdfBytes,
	}

	if baPayload != nil {
		baPdfBytes, pErr := services.BuildBeritaAcaraPDF(ctx, baPayload)
		if pErr != nil {
			return nil, "", "", fmt.Errorf("build berita acara pdf: %w", pErr)
		}
		baPdfFilename := fmt.Sprintf("Berita_Acara_%s_%02d_%04d.pdf", sanitizeFilename(baUser.Username), month, year)
		filesToZip[baDocxFilename] = baDocxBytes
		filesToZip[baPdfFilename] = baPdfBytes
	}

	zipBytes, zErr := createZipArchive(filesToZip)
	if zErr != nil {
		return nil, "", "", zErr
	}
	return zipBytes, fmt.Sprintf("%s_with_PDF.zip", baseName), "application/zip", nil
}

// PackageTimesheetBundle packages the timesheet workbook, optional Berita Acara, and optional PDFs into a single file or ZIP archive.
func PackageTimesheetBundle(ctx context.Context, tsSvc TimesheetService, userID uint, month, year int, includePDF bool) ([]byte, string, string, error) {
	if bundler, ok := tsSvc.(TimesheetBundler); ok {
		return bundler.PackageTimesheetBundle(ctx, userID, month, year, includePDF)
	}
	return fallbackPackageBundle(ctx, tsSvc, userID, month, year, includePDF)
}

func fallbackPackageBundle(ctx context.Context, tsSvc TimesheetService, userID uint, month, year int, includePDF bool) ([]byte, string, string, error) {
	out, xlsxFilename, err := tsSvc.GenerateWorkbook(ctx, userID, month, year)
	if err != nil {
		return nil, "", "", err
	}
	baseName := strings.TrimSuffix(xlsxFilename, filepath.Ext(xlsxFilename))

	baBytes, baFilename, _ := tsSvc.GenerateBeritaAcara(ctx, userID, month, year)
	hasBA := len(baBytes) > 0

	if !includePDF {
		if !hasBA {
			return out, xlsxFilename, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil
		}
		zipBytes, zErr := createZipArchive(map[string][]byte{
			xlsxFilename: out,
			baFilename:   baBytes,
		})
		if zErr != nil {
			return nil, "", "", zErr
		}
		return zipBytes, fmt.Sprintf("%s_and_Berita_Acara.zip", baseName), "application/zip", nil
	}

	// includePDF == true
	pdfGen, ok := tsSvc.(TimesheetPDFGenerator)
	if !ok {
		return nil, "", "", fmt.Errorf("timesheet service does not support PDF generation")
	}

	tsPdfBytes, tsPdfFilename, err := pdfGen.GenerateTimesheetPDF(ctx, userID, month, year)
	if err != nil {
		return nil, "", "", fmt.Errorf("generate timesheet pdf: %w", err)
	}
	if tsPdfFilename == "" {
		tsPdfFilename = fmt.Sprintf("%s.pdf", baseName)
	}

	filesToZip := map[string][]byte{
		xlsxFilename:  out,
		tsPdfFilename: tsPdfBytes,
	}

	if hasBA {
		baPdfBytes, baPdfFilename, baPdfErr := pdfGen.GenerateBeritaAcaraPDF(ctx, userID, month, year)
		if baPdfErr != nil {
			return nil, "", "", fmt.Errorf("generate berita acara pdf: %w", baPdfErr)
		}
		if baPdfFilename == "" {
			baBase := strings.TrimSuffix(baFilename, filepath.Ext(baFilename))
			baPdfFilename = fmt.Sprintf("%s.pdf", baBase)
		}
		filesToZip[baFilename] = baBytes
		filesToZip[baPdfFilename] = baPdfBytes
	}

	zipBytes, zErr := createZipArchive(filesToZip)
	if zErr != nil {
		return nil, "", "", zErr
	}
	return zipBytes, fmt.Sprintf("%s_with_PDF.zip", baseName), "application/zip", nil
}

func createZipArchive(files map[string][]byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		w, err := zw.Create(name)
		if err != nil {
			return nil, fmt.Errorf("create zip entry %s: %w", name, err)
		}
		if _, err := w.Write(files[name]); err != nil {
			return nil, fmt.Errorf("write zip entry %s: %w", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close zip writer: %w", err)
	}
	return buf.Bytes(), nil
}

func (s *timesheetService) validateTimesheetPeriod(ctx context.Context, month, year int) (*time.Location, error) {
	if month < 1 || month > 12 || year < 2000 || year > 2100 {
		return nil, domain.NewUserError(domain.ErrInvalidInput, "Invalid month or year")
	}
	loc := jakartaLocation()
	now := time.Now()
	if s.nowFunc != nil {
		now = s.nowFunc()
	}
	if !domain.IsAdminFromContext(ctx) {
		if err := domain.ValidateTimesheetPeriod(month, year, now, loc); err != nil {
			return nil, err
		}
	}
	return loc, nil
}

func (s *timesheetService) resolveCompanyCode(ctx context.Context, user *models.User) (string, error) {
	if user.CompanyRel != nil && user.CompanyRel.Code != "" {
		return user.CompanyRel.Code, nil
	}
	if user.CompanyID != nil && *user.CompanyID != 0 && s.masterRepo != nil {
		if comp, cerr := s.masterRepo.FindCompanyByID(ctx, *user.CompanyID); cerr == nil && comp != nil && comp.Code != "" {
			return comp.Code, nil
		}
	}
	if user.Company != "" {
		return user.Company, nil
	}
	return "", domain.NewUserError(domain.ErrInvalidInput, "User has no company assigned")
}

func (s *timesheetService) fetchHolidaysMap(ctx context.Context, year, month int) map[int]string {
	holidays := make(map[int]string)
	if s.masterRepo != nil {
		if hs, herr := s.masterRepo.ListHolidaysByMonth(ctx, year, month); herr == nil {
			for _, h := range hs {
				holidays[h.Date.Day()] = h.Description
			}
		}
	}
	return holidays
}

func (s *timesheetService) fetchActiveApprovers(ctx context.Context) []models.Approver {
	if s.masterRepo != nil {
		activeStatus := true
		approvers, _ := s.masterRepo.ListApprovers(ctx, "", &activeStatus)
		return approvers
	}
	return nil
}

func (s *timesheetService) resolveWitnessName(ctx context.Context, userID uint, month, year int, baActivities []models.DailyActivity) string {
	if w, werr := s.activityRepo.GetMonthlyWitness(ctx, userID, month, year); werr == nil && w != nil && w.Name != "" {
		return w.Name
	}
	for _, a := range baActivities {
		if a.Witness != nil && a.Witness.Name != "" {
			return a.Witness.Name
		}
		if a.WitnessID != nil && *a.WitnessID != 0 {
			if w, werr := s.activityRepo.FindWitnessByID(ctx, *a.WitnessID); werr == nil && w != nil && w.Name != "" {
				return w.Name
			}
		}
	}
	return ""
}

func buildBeritaAcaraItems(baActivities []models.DailyActivity) []services.BeritaAcaraItem {
	items := make([]services.BeritaAcaraItem, 0, len(baActivities))
	for _, a := range baActivities {
		items = append(items, services.BeritaAcaraItem{
			Date:      a.Date,
			Day:       services.IndonesianDayName(a.Date.Weekday()),
			StartTime: a.StartTime,
			EndTime:   a.EndTime,
			Notes:     a.BeritaAcaraNotes,
		})
	}
	return items
}

func (s *timesheetService) UpsertOvertime(ctx context.Context, userID uint, req *request.OvertimeRequest) error {
	if req == nil {
		return domain.NewUserError(domain.ErrInvalidInput, "Request payload is required")
	}
	if err := validateWorkingHours(req.StartTime, req.EndTime); err != nil {
		return err
	}
	loc := jakartaLocation()
	date, err := time.ParseInLocation(dateFormatYYYYMMDD, req.Date, loc)
	if err != nil {
		return domain.NewUserError(domain.ErrInvalidInput, "Invalid date format, expected YYYY-MM-DD")
	}

	now := time.Now()
	if s.nowFunc != nil {
		now = s.nowFunc()
	}

	if !domain.IsAdminFromContext(ctx) {
		if err := domain.ValidateTimesheetDate(date, now, loc); err != nil {
			return err
		}
	}

	var entry *models.OvertimeEntry
	if req.ID != 0 {
		entry, _ = s.overtimeRepo.FindActiveByID(ctx, req.ID, userID)
	}
	if entry == nil {
		entry, _ = s.overtimeRepo.FindByUserAndDate(ctx, userID, date)
	}

	if entry != nil && entry.ID != 0 {
		entry.Date = date
		entry.StartTime = req.StartTime
		entry.EndTime = req.EndTime
		entry.TaskDescription = req.TaskDescription
		entry.TeamLeaderID = req.TeamLeaderID
		entry.DepartmentHeadID = req.DepartmentHeadID
		entry.UpdatedAt = time.Now()
		return s.overtimeRepo.Update(ctx, entry)
	}

	newEntry := &models.OvertimeEntry{
		UserID:           userID,
		Date:             date,
		StartTime:        req.StartTime,
		EndTime:          req.EndTime,
		TaskDescription:  req.TaskDescription,
		TeamLeaderID:     req.TeamLeaderID,
		DepartmentHeadID: req.DepartmentHeadID,
		IsActive:         true,
	}
	return s.overtimeRepo.Create(ctx, newEntry)
}

func (s *timesheetService) ListMonthlyOvertimes(ctx context.Context, userID uint, month int, year int) ([]response.OvertimeResponse, error) {
	loc := jakartaLocation()
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, 0)

	overtimes, err := s.overtimeRepo.FindByUserAndMonth(ctx, userID, start, end)
	if err != nil {
		return nil, err
	}

	resp := make([]response.OvertimeResponse, len(overtimes))
	for i, ot := range overtimes {
		var tlName, dhName string
		if ot.TeamLeader != nil {
			tlName = ot.TeamLeader.Name
		}
		if ot.DepartmentHead != nil {
			dhName = ot.DepartmentHead.Name
		}
		resp[i] = response.OvertimeResponse{
			ID:                 ot.ID,
			Date:               ot.Date,
			StartTime:          ot.StartTime,
			EndTime:            ot.EndTime,
			TaskDescription:    ot.TaskDescription,
			TeamLeaderID:       ot.TeamLeaderID,
			TeamLeaderName:     tlName,
			DepartmentHeadID:   ot.DepartmentHeadID,
			DepartmentHeadName: dhName,
		}
	}
	return resp, nil
}

func (s *timesheetService) DeleteOvertime(ctx context.Context, id uint, userID uint) error {
	entry, err := s.overtimeRepo.FindActiveByID(ctx, id, userID)
	if err != nil || entry == nil {
		return domain.NewUserError(domain.ErrNotFound, "Overtime entry not found")
	}

	now := time.Now()
	if s.nowFunc != nil {
		now = s.nowFunc()
	}

	if !domain.IsAdminFromContext(ctx) {
		if err := domain.ValidateTimesheetDate(entry.Date, now, jakartaLocation()); err != nil {
			return err
		}
	}
	return s.overtimeRepo.SoftDelete(ctx, id, userID)
}
