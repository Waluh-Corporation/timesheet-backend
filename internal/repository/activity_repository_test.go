package repository_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"timesheet-backend/internal/repository"
	"timesheet-backend/models"
)

func TestActivityRepository(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	repo := repository.NewActivityRepository(tx)
	ctx := context.Background()

	// Ensure test user
	user := &models.User{
		Username:     "act_repo_user_unique",
		Email:        "act_repo_user@example.com",
		Name:         "Act Repo User",
		Role:         models.RoleUser,
		PasswordHash: "dummyhash",
		IsActive:     true,
	}
	if err := tx.Create(user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Ensure test project
	proj := &models.Project{
		Code:        "ACT-PRJ-01",
		Name:        "Test Act Project",
		AppImpacted: "Backend",
		IsActive:    true,
	}
	if err := tx.Create(proj).Error; err != nil {
		t.Fatalf("failed to create test project: %v", err)
	}

	date, _ := time.Parse("2006-01-02", "2026-09-25")
	act := &models.DailyActivity{
		UserID:       user.ID,
		Date:         date,
		StartTime:    "08:30",
		EndTime:      "17:30",
		Status:       "P",
		Activity:     "Repository test activity",
		ProjectRefID: &proj.ID,
		IsActive:     true,
	}

	// 1. Create
	if err := repo.Create(ctx, act); err != nil {
		t.Fatalf("repo.Create failed: %v", err)
	}
	if act.ID == 0 {
		t.Fatal("expected activity ID to be non-zero")
	}

	// 2. FindActiveByID
	found, err := repo.FindActiveByID(ctx, act.ID)
	if err != nil {
		t.Fatalf("repo.FindActiveByID failed: %v", err)
	}
	if found.Activity != act.Activity {
		t.Errorf("expected activity %q, got %q", act.Activity, found.Activity)
	}

	// 3. FindActiveByUserAndDate
	foundByDate, err := repo.FindActiveByUserAndDate(ctx, user.ID, date)
	if err != nil {
		t.Fatalf("repo.FindActiveByUserAndDate failed: %v", err)
	}
	if foundByDate.ID != act.ID {
		t.Errorf("expected ID %d, got %d", act.ID, foundByDate.ID)
	}

	// 4. Update
	foundByDate.Activity = "Updated repo activity"
	if err := repo.Update(ctx, foundByDate); err != nil {
		t.Fatalf("repo.Update failed: %v", err)
	}

	refreshed, err := repo.FindActiveByID(ctx, act.ID)
	if err != nil || refreshed.Activity != "Updated repo activity" {
		t.Fatalf("repo.Update not reflected, got: %v, err: %v", refreshed, err)
	}

	// 5. ListActiveByUser with various filters
	// 5a. Default filter
	filter := repository.ActivityFilter{
		Page:  1,
		Limit: 10,
	}
	list, total, err := repo.ListActiveByUser(ctx, user.ID, filter)
	if err != nil {
		t.Fatalf("repo.ListActiveByUser failed: %v", err)
	}
	if total < 1 || len(list) < 1 {
		t.Fatalf("expected at least 1 record in list, got total=%d, len=%d", total, len(list))
	}

	// 5b. StartDate & EndDate filter, with limit capping > 100 and sortOrder asc
	startDate := date.AddDate(0, 0, -1)
	endDate := date.AddDate(0, 0, 1)
	filterDates := repository.ActivityFilter{
		StartDate: &startDate,
		EndDate:   &endDate,
		Status:    "P",
		SortOrder: "asc",
		Page:      1,
		Limit:     150, // tests limit > 100 capping
	}
	listDates, totalDates, err := repo.ListActiveByUser(ctx, user.ID, filterDates)
	if err != nil || totalDates < 1 || len(listDates) < 1 {
		t.Fatalf("ListActiveByUser with date range failed: %v", err)
	}

	// 5c. Month & Year filter with sortOrder desc
	month := int(date.Month())
	year := date.Year()
	filterMonthYear := repository.ActivityFilter{
		Month:     &month,
		Year:      &year,
		SortOrder: "desc",
		Page:      1,
		Limit:     10,
	}
	listMY, totalMY, err := repo.ListActiveByUser(ctx, user.ID, filterMonthYear)
	if err != nil || totalMY < 1 || len(listMY) < 1 {
		t.Fatalf("ListActiveByUser with Month/Year failed: %v", err)
	}

	// 5d. Year only filter with fallback sortOrder
	filterYearOnly := repository.ActivityFilter{
		Year:      &year,
		SortOrder: "invalid_sort", // tests default fallback to "desc"
		Page:      1,
		Limit:     10,
	}
	listY, totalY, err := repo.ListActiveByUser(ctx, user.ID, filterYearOnly)
	if err != nil || totalY < 1 || len(listY) < 1 {
		t.Fatalf("ListActiveByUser with Year only failed: %v", err)
	}

	// 5e. IsCurrentMonthDefault and IsAll
	filterDefaultMonth := repository.ActivityFilter{
		IsCurrentMonthDefault: true,
		IsAll:                 true,
	}
	_, _, err = repo.ListActiveByUser(ctx, user.ID, filterDefaultMonth)
	if err != nil {
		t.Fatalf("ListActiveByUser with IsCurrentMonthDefault failed: %v", err)
	}

	// 6. ValidateStatus
	isValid, err := repo.ValidateStatus(ctx, "P")
	if err != nil || !isValid {
		t.Error("expected status P to be valid")
	}
	isInvalid, err := repo.ValidateStatus(ctx, "NON_EXISTENT_STATUS_XYZ")
	if err != nil || isInvalid {
		t.Error("expected non-existent status to be invalid")
	}

	// 7. FindActiveProjectByRefID
	foundProj, err := repo.FindActiveProjectByRefID(ctx, proj.ID)
	if err != nil || foundProj.Code != proj.Code {
		t.Fatalf("repo.FindActiveProjectByRefID failed: %v", err)
	}
	_, err = repo.FindActiveProjectByRefID(ctx, 999999)
	if err == nil {
		t.Error("expected error for non-existent project refID, got nil")
	}

	// 8. FindActiveProjectByCodeOrName
	// 8a. By name
	foundByName, err := repo.FindActiveProjectByCodeOrName(ctx, "", proj.Name)
	if err != nil || foundByName.ID != proj.ID {
		t.Fatalf("repo.FindActiveProjectByCodeOrName by name failed: %v", err)
	}
	// 8b. By numeric ID string
	foundByNumID, err := repo.FindActiveProjectByCodeOrName(ctx, strconv.Itoa(int(proj.ID)), "")
	if err != nil || foundByNumID.ID != proj.ID {
		t.Fatalf("repo.FindActiveProjectByCodeOrName by numeric ID failed: %v", err)
	}
	// 8c. By code only
	foundByCode, err := repo.FindActiveProjectByCodeOrName(ctx, proj.Code, "")
	if err != nil || foundByCode.ID != proj.ID {
		t.Fatalf("repo.FindActiveProjectByCodeOrName by code failed: %v", err)
	}
	// 8d. By both code and name
	foundByBoth, err := repo.FindActiveProjectByCodeOrName(ctx, proj.Code, proj.Name)
	if err != nil || foundByBoth.ID != proj.ID {
		t.Fatalf("repo.FindActiveProjectByCodeOrName by both failed: %v", err)
	}
	// 8e. Non-existent project
	_, err = repo.FindActiveProjectByCodeOrName(ctx, "NON_EXISTENT_CODE", "NON_EXISTENT_NAME")
	if err == nil {
		t.Error("expected error for non-existent project code/name, got nil")
	}

	// 9. FindActiveByID not found
	_, err = repo.FindActiveByID(ctx, 999999)
	if err == nil {
		t.Error("expected error for non-existent activity ID, got nil")
	}

	// 10. FindActiveByUserAndDate not found
	_, err = repo.FindActiveByUserAndDate(ctx, user.ID, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	if err == nil {
		t.Error("expected error for non-existent user and date, got nil")
	}

	// 11. FindPreviousActiveByUserAndDate
	// 11a. Should find the activity created above when querying a later date
	prevAct, err := repo.FindPreviousActiveByUserAndDate(ctx, user.ID, date.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("expected to find previous activity, got err: %v", err)
	}
	if prevAct.ID != act.ID {
		t.Errorf("expected previous activity ID %d, got %d", act.ID, prevAct.ID)
	}

	// 11b. When querying the exact same date (strictly before), it should not match
	_, err = repo.FindPreviousActiveByUserAndDate(ctx, user.ID, date)
	if err == nil {
		t.Error("expected error when querying with same date, got nil")
	}

	// 11c. Non-existent user
	_, err = repo.FindPreviousActiveByUserAndDate(ctx, 999999, date.AddDate(0, 0, 1))
	if err == nil {
		t.Error("expected error for non-existent user, got nil")
	}

	// 12. GetLatestActivityUpdateTime
	tUpdate, err := repo.GetLatestActivityUpdateTime(ctx, user.ID, int(date.Month()), date.Year())
	if err != nil {
		t.Fatalf("repo.GetLatestActivityUpdateTime failed: %v", err)
	}
	if tUpdate.IsZero() {
		t.Error("expected non-zero latest update time")
	}
	tUpdateEmpty, err := repo.GetLatestActivityUpdateTime(ctx, 999999, 1, 2026)
	if err != nil || !tUpdateEmpty.IsZero() {
		t.Errorf("expected zero update time for empty period, got %v, err: %v", tUpdateEmpty, err)
	}

	// 13. Witness setup and FindWitnessByID
	witnessUser := &models.User{
		Username:     "witness_repo_unique",
		Email:        "witness_repo@example.com",
		Name:         "Witness Repo User",
		Role:         models.RoleUser,
		PasswordHash: "dummyhash",
		IsActive:     true,
	}
	if err := tx.Create(witnessUser).Error; err != nil {
		t.Fatalf("failed to create witness user: %v", err)
	}
	wFound, err := repo.FindWitnessByID(ctx, witnessUser.ID)
	if err != nil || wFound.ID != witnessUser.ID {
		t.Fatalf("repo.FindWitnessByID failed: %v", err)
	}
	_, err = repo.FindWitnessByID(ctx, 999999)
	if err == nil {
		t.Error("expected error for non-existent witness ID, got nil")
	}

	// 14. GetEligibleWitnesses
	eligible, err := repo.GetEligibleWitnesses(ctx, user.ID)
	if err != nil {
		t.Fatalf("repo.GetEligibleWitnesses failed: %v", err)
	}
	foundWitnessInEligible := false
	for _, u := range eligible {
		if u.ID == witnessUser.ID {
			foundWitnessInEligible = true
		}
		if u.ID == user.ID {
			t.Errorf("expected excluded user %d not to be in eligible list", user.ID)
		}
	}
	if !foundWitnessInEligible {
		t.Error("expected witnessUser to be in eligible witnesses list")
	}

	// 15. GetMonthlyWitness (initially nil when no BA activities)
	mWitness, err := repo.GetMonthlyWitness(ctx, user.ID, int(date.Month()), date.Year())
	if err != nil || mWitness != nil {
		t.Errorf("expected nil monthly witness before BA activity, got %v, err: %v", mWitness, err)
	}

	// 16. UpdateMonthlyWitness and GetMonthlyWitness
	// Create BA activity
	baDate := date.AddDate(0, 0, 2)
	baAct := &models.DailyActivity{
		UserID:           user.ID,
		Date:             baDate,
		StartTime:        "09:00",
		EndTime:          "18:00",
		Status:           "P",
		Activity:         "BA test activity",
		IsActive:         true,
		IsBeritaAcara:    true,
		WitnessID:        &witnessUser.ID,
		BeritaAcaraNotes: "Notes",
	}
	if err := repo.Create(ctx, baAct); err != nil {
		t.Fatalf("failed to create BA activity: %v", err)
	}

	// Query monthly witness now
	mWitnessAfter, err := repo.GetMonthlyWitness(ctx, user.ID, int(date.Month()), date.Year())
	if err != nil || mWitnessAfter == nil || mWitnessAfter.ID != witnessUser.ID {
		t.Fatalf("expected monthly witness %d, got %v, err: %v", witnessUser.ID, mWitnessAfter, err)
	}

	// Update monthly witness to another witness
	anotherWitness := &models.User{
		Username:     "another_witness_unique",
		Email:        "another_witness@example.com",
		Name:         "Another Witness User",
		Role:         models.RoleUser,
		PasswordHash: "dummyhash",
		IsActive:     true,
	}
	if err := tx.Create(anotherWitness).Error; err != nil {
		t.Fatalf("failed to create another witness: %v", err)
	}
	rowsAffected, err := repo.UpdateMonthlyWitness(ctx, user.ID, int(date.Month()), date.Year(), anotherWitness.ID)
	if err != nil {
		t.Fatalf("repo.UpdateMonthlyWitness failed: %v", err)
	}
	if rowsAffected < 1 {
		t.Errorf("expected at least 1 row affected, got %d", rowsAffected)
	}

	mWitnessUpdated, err := repo.GetMonthlyWitness(ctx, user.ID, int(date.Month()), date.Year())
	if err != nil || mWitnessUpdated == nil || mWitnessUpdated.ID != anotherWitness.ID {
		t.Fatalf("expected updated monthly witness %d, got %v, err: %v", anotherWitness.ID, mWitnessUpdated, err)
	}
}
