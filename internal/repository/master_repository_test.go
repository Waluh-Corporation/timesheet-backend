package repository_test

import (
	"context"
	"testing"

	"timesheet-backend/internal/repository"
	"timesheet-backend/models"
)

func TestMasterRepository_All(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	repo := repository.NewMasterRepository(tx)
	ctx := context.Background()

	// --- Approvers ---
	t.Run("Approvers", func(t *testing.T) {
		appr := &models.Approver{
			Name:     "Test Approver 1",
			RoleType: models.ApproverRoleTeamLeader,
			Title:    "Tech Lead",
			IsActive: true,
		}
		if err := repo.CreateApprover(ctx, appr); err != nil {
			t.Fatalf("CreateApprover failed: %v", err)
		}
		if appr.ID == 0 {
			t.Fatal("expected non-zero ID")
		}

		found, err := repo.FindApproverByID(ctx, appr.ID)
		if err != nil || found.Name != "Test Approver 1" {
			t.Fatalf("FindApproverByID failed: %v", err)
		}

		active := true
		list, err := repo.ListApprovers(ctx, "team_leader", &active)
		if err != nil || len(list) == 0 {
			t.Fatalf("ListApprovers failed: %v", err)
		}

		appr.Name = "Updated Approver 1"
		if err := repo.UpdateApprover(ctx, appr); err != nil {
			t.Fatalf("UpdateApprover failed: %v", err)
		}

		if err := repo.SoftDeleteApprover(ctx, appr.ID); err != nil {
			t.Fatalf("SoftDeleteApprover failed: %v", err)
		}
		if _, err := repo.FindApproverByID(ctx, appr.ID); err == nil {
			t.Fatal("expected error finding soft-deleted approver")
		}
	})

	// --- Companies ---
	t.Run("Companies", func(t *testing.T) {
		comp := &models.Company{
			Code:     "testcomp",
			Name:     "Test Company",
			IsActive: true,
		}
		if err := repo.CreateCompany(ctx, comp); err != nil {
			t.Fatalf("CreateCompany failed: %v", err)
		}

		found, err := repo.FindCompanyByID(ctx, comp.ID)
		if err != nil || found.Code != "testcomp" {
			t.Fatalf("FindCompanyByID failed: %v", err)
		}

		byCode, err := repo.FindCompanyByCode(ctx, "TESTCOMP")
		if err != nil || byCode.ID != comp.ID {
			t.Fatalf("FindCompanyByCode failed: %v", err)
		}

		active := true
		comps, err := repo.ListCompanies(ctx, &active)
		if err != nil || len(comps) == 0 {
			t.Fatalf("ListCompanies failed: %v", err)
		}

		comp.Name = "Updated Test Company"
		if err := repo.UpdateCompany(ctx, comp); err != nil {
			t.Fatalf("UpdateCompany failed: %v", err)
		}

		if err := repo.SoftDeleteCompany(ctx, comp.ID); err != nil {
			t.Fatalf("SoftDeleteCompany failed: %v", err)
		}
		if _, err := repo.FindCompanyByID(ctx, comp.ID); err == nil {
			t.Fatal("expected error finding soft-deleted company")
		}
	})

	// --- Sites ---
	t.Run("Sites", func(t *testing.T) {
		site := &models.Site{
			Code:     "testsite",
			Name:     "Test Site",
			IsActive: true,
		}
		if err := repo.CreateSite(ctx, site); err != nil {
			t.Fatalf("CreateSite failed: %v", err)
		}

		found, err := repo.FindSiteByID(ctx, site.ID)
		if err != nil || found.Code != "testsite" {
			t.Fatalf("FindSiteByID failed: %v", err)
		}

		byCode, err := repo.FindSiteByCode(ctx, "TESTSITE")
		if err != nil || byCode.ID != site.ID {
			t.Fatalf("FindSiteByCode failed: %v", err)
		}

		active := true
		sites, err := repo.ListSites(ctx, &active)
		if err != nil || len(sites) == 0 {
			t.Fatalf("ListSites failed: %v", err)
		}

		site.Name = "Updated Test Site"
		if err := repo.UpdateSite(ctx, site); err != nil {
			t.Fatalf("UpdateSite failed: %v", err)
		}

		if err := repo.SoftDeleteSite(ctx, site.ID); err != nil {
			t.Fatalf("SoftDeleteSite failed: %v", err)
		}
		if _, err := repo.FindSiteByID(ctx, site.ID); err == nil {
			t.Fatal("expected error finding soft-deleted site")
		}
	})

	// --- Divisions ---
	t.Run("Divisions", func(t *testing.T) {
		div := &models.Division{
			Code:     "testdiv",
			Name:     "Test Division",
			IsActive: true,
		}
		if err := repo.CreateDivision(ctx, div); err != nil {
			t.Fatalf("CreateDivision failed: %v", err)
		}

		found, err := repo.FindDivisionByID(ctx, div.ID)
		if err != nil || found.Code != "testdiv" {
			t.Fatalf("FindDivisionByID failed: %v", err)
		}

		byCode, err := repo.FindDivisionByCode(ctx, "TESTDIV")
		if err != nil || byCode.ID != div.ID {
			t.Fatalf("FindDivisionByCode failed: %v", err)
		}

		byName, err := repo.FindActiveDivisionByCodeOrName(ctx, "Test Division")
		if err != nil || byName.ID != div.ID {
			t.Fatalf("FindActiveDivisionByCodeOrName failed: %v", err)
		}

		active := true
		divs, err := repo.ListDivisions(ctx, &active)
		if err != nil || len(divs) == 0 {
			t.Fatalf("ListDivisions failed: %v", err)
		}

		div.Name = "Updated Test Division"
		if err := repo.UpdateDivision(ctx, div); err != nil {
			t.Fatalf("UpdateDivision failed: %v", err)
		}

		if err := repo.SoftDeleteDivision(ctx, div.ID); err != nil {
			t.Fatalf("SoftDeleteDivision failed: %v", err)
		}
		if _, err := repo.FindDivisionByID(ctx, div.ID); err == nil {
			t.Fatal("expected error finding soft-deleted division")
		}
	})

	// --- Departments ---
	t.Run("Departments", func(t *testing.T) {
		div := &models.Division{
			Code:     "divdept",
			Name:     "Division Dept Parent",
			IsActive: true,
		}
		_ = repo.CreateDivision(ctx, div)

		dept := &models.Department{
			Code:       "TESTDEPT",
			Name:       "Test Department",
			Division:   div.Name,
			DivisionID: &div.ID,
			IsActive:   true,
		}
		if err := repo.CreateDepartment(ctx, dept); err != nil {
			t.Fatalf("CreateDepartment failed: %v", err)
		}

		found, err := repo.FindDepartmentByID(ctx, dept.ID)
		if err != nil || found.Code != "TESTDEPT" {
			t.Fatalf("FindDepartmentByID failed: %v", err)
		}

		byName, err := repo.FindDepartmentByName(ctx, "TEST DEPARTMENT")
		if err != nil || byName.ID != dept.ID {
			t.Fatalf("FindDepartmentByName failed: %v", err)
		}

		active := true
		depts, err := repo.ListDepartments(ctx, &div.ID, "Division Dept Parent", &active)
		if err != nil || len(depts) == 0 {
			t.Fatalf("ListDepartments failed: %v", err)
		}

		dept.Name = "Updated Test Department"
		if err := repo.UpdateDepartment(ctx, dept); err != nil {
			t.Fatalf("UpdateDepartment failed: %v", err)
		}

		if err := repo.SoftDeleteDepartment(ctx, dept.ID); err != nil {
			t.Fatalf("SoftDeleteDepartment failed: %v", err)
		}
		if _, err := repo.FindDepartmentByID(ctx, dept.ID); err == nil {
			t.Fatal("expected error finding soft-deleted department")
		}
	})

	// --- Projects & Statuses ---
	t.Run("ProjectsAndStatuses", func(t *testing.T) {
		projs, err := repo.ListProjects(ctx, true)
		if err != nil {
			t.Fatalf("ListProjects failed: %v", err)
		}
		_ = projs

		statuses, err := repo.ListActivityStatuses(ctx)
		if err != nil {
			t.Fatalf("ListActivityStatuses failed: %v", err)
		}
		_ = statuses
	})
}
