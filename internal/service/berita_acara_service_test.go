package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"timesheet-backend/dto/request"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/models"
)

type mockBeritaAcaraRepo struct {
	items     map[uint]*models.BeritaAcara
	nextID    uint
	createErr error
	updateErr error
	listErr   error
}

func newMockBeritaAcaraRepo() *mockBeritaAcaraRepo {
	return &mockBeritaAcaraRepo{
		items:  make(map[uint]*models.BeritaAcara),
		nextID: 1,
	}
}

func (m *mockBeritaAcaraRepo) FindActiveByID(ctx context.Context, id uint) (*models.BeritaAcara, error) {
	item, ok := m.items[id]
	if !ok || !item.IsActive {
		return nil, domain.ErrNotFound
	}
	return item, nil
}

func (m *mockBeritaAcaraRepo) FindActiveByUserAndDate(ctx context.Context, userID uint, date time.Time) (*models.BeritaAcara, error) {
	for _, it := range m.items {
		if it.UserID == userID && it.Date.Equal(date) && it.IsActive {
			return it, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *mockBeritaAcaraRepo) Create(ctx context.Context, item *models.BeritaAcara) error {
	if m.createErr != nil {
		return m.createErr
	}
	item.ID = m.nextID
	m.nextID++
	m.items[item.ID] = item
	return nil
}

func (m *mockBeritaAcaraRepo) Update(ctx context.Context, item *models.BeritaAcara) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.items[item.ID] = item
	return nil
}

func (m *mockBeritaAcaraRepo) ListActiveByUser(ctx context.Context, userID uint, filter repository.BeritaAcaraFilter) ([]models.BeritaAcara, int64, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	var res []models.BeritaAcara
	for _, it := range m.items {
		if it.UserID == userID && it.IsActive {
			res = append(res, *it)
		}
	}
	return res, int64(len(res)), nil
}

func (m *mockBeritaAcaraRepo) FindActiveForMonth(ctx context.Context, userID uint, year, month int) ([]models.BeritaAcara, error) {
	var res []models.BeritaAcara
	for _, it := range m.items {
		if it.UserID == userID && it.IsActive && it.Date.Year() == year && int(it.Date.Month()) == month {
			res = append(res, *it)
		}
	}
	return res, nil
}

func TestBeritaAcaraService(t *testing.T) {
	ctx := context.Background()

	t.Run("Upsert invalid date format", func(t *testing.T) {
		repo := newMockBeritaAcaraRepo()
		svc := NewBeritaAcaraService(repo)

		err := svc.UpsertBeritaAcara(ctx, 1, &request.BeritaAcaraRequest{
			Date:       "invalid-date",
			Keterangan: "Some note",
		})
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("Upsert empty keterangan", func(t *testing.T) {
		repo := newMockBeritaAcaraRepo()
		svc := NewBeritaAcaraService(repo)

		err := svc.UpsertBeritaAcara(ctx, 1, &request.BeritaAcaraRequest{
			Date:       "2026-09-01",
			Keterangan: "   ",
		})
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("Upsert create and then update (upsert flow)", func(t *testing.T) {
		repo := newMockBeritaAcaraRepo()
		svc := NewBeritaAcaraService(repo)

		// 1. First insert
		err := svc.UpsertBeritaAcara(ctx, 1, &request.BeritaAcaraRequest{
			Date:       "2026-09-01", // Tuesday -> Selasa
			StartTime:  "08:00",
			EndTime:    "17:00",
			Keterangan: "Lupa absen masuk",
		})
		if err != nil {
			t.Fatalf("first upsert failed: %v", err)
		}

		list, _, _ := svc.ListBeritaAcara(ctx, 1, repository.BeritaAcaraFilter{})
		if len(list) != 1 {
			t.Fatalf("expected 1 item, got %d", len(list))
		}
		if list[0].Day != "Selasa" {
			t.Errorf("expected day 'Selasa', got %s", list[0].Day)
		}
		if list[0].Keterangan != "Lupa absen masuk" {
			t.Errorf("expected keterangan 'Lupa absen masuk', got %s", list[0].Keterangan)
		}

		// 2. Upsert same date -> should update
		err = svc.UpsertBeritaAcara(ctx, 1, &request.BeritaAcaraRequest{
			Date:       "2026-09-01",
			StartTime:  "08:30",
			EndTime:    "17:30",
			Keterangan: "Lupa absen masuk dan pulang",
		})
		if err != nil {
			t.Fatalf("second upsert failed: %v", err)
		}

		listAfter, _, _ := svc.ListBeritaAcara(ctx, 1, repository.BeritaAcaraFilter{})
		if len(listAfter) != 1 {
			t.Fatalf("expected 1 item after update, got %d", len(listAfter))
		}
		if listAfter[0].Keterangan != "Lupa absen masuk dan pulang" {
			t.Errorf("expected updated keterangan, got %s", listAfter[0].Keterangan)
		}
		if listAfter[0].StartTime != "08:30" {
			t.Errorf("expected updated startTime '08:30', got %s", listAfter[0].StartTime)
		}
	})

	t.Run("GetBeritaAcara not found and forbidden", func(t *testing.T) {
		repo := newMockBeritaAcaraRepo()
		svc := NewBeritaAcaraService(repo)

		// Not found
		_, err := svc.GetBeritaAcara(ctx, 1, 999)
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}

		// Create record for user 1
		_ = svc.UpsertBeritaAcara(ctx, 1, &request.BeritaAcaraRequest{
			Date:       "2026-09-02",
			Keterangan: "Test forbidden",
		})

		// User 2 tries to access user 1's record
		_, err = svc.GetBeritaAcara(ctx, 2, 1)
		if !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("expected ErrForbidden, got %v", err)
		}

		// User 1 accesses user 1's record -> OK
		detail, err := svc.GetBeritaAcara(ctx, 1, 1)
		if err != nil || detail == nil {
			t.Fatalf("expected detail, got err: %v", err)
		}
		if detail.Keterangan != "Test forbidden" {
			t.Errorf("expected 'Test forbidden', got %s", detail.Keterangan)
		}
	})

	t.Run("FindActiveForMonth returns only month records", func(t *testing.T) {
		repo := newMockBeritaAcaraRepo()
		svc := NewBeritaAcaraService(repo)

		_ = svc.UpsertBeritaAcara(ctx, 1, &request.BeritaAcaraRequest{Date: "2026-09-05", Keterangan: "Sept record"})
		_ = svc.UpsertBeritaAcara(ctx, 1, &request.BeritaAcaraRequest{Date: "2026-10-05", Keterangan: "Oct record"})

		sept, _ := svc.FindActiveForMonth(ctx, 1, 2026, 9)
		if len(sept) != 1 || sept[0].Keterangan != "Sept record" {
			t.Errorf("expected 1 september record, got %d", len(sept))
		}
	})
}
