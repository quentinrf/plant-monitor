package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/quentinrf/plant-monitor/services/light-service/internal/domain"
)

func newTestRepo(t *testing.T) *ReadingRepository {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	repo, err := NewReadingRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to create SQLite repo: %v", err)
	}
	t.Cleanup(func() { repo.Close() })
	return repo
}

func TestSaveAndGetReading(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	reading, err := domain.NewLightReading(500.0)
	if err != nil {
		t.Fatalf("unexpected error creating reading: %v", err)
	}

	if err := repo.SaveReading(ctx, reading); err != nil {
		t.Fatalf("SaveReading failed: %v", err)
	}
	if reading.ID == 0 {
		t.Fatal("expected ID to be set after save")
	}

	got, err := repo.GetReading(ctx, reading.ID)
	if err != nil {
		t.Fatalf("GetReading failed: %v", err)
	}
	if got.Lux != reading.Lux {
		t.Errorf("got lux %v, want %v", got.Lux, reading.Lux)
	}
}

func TestGetLatestReading_Empty(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	_, err := repo.GetLatestReading(ctx)
	if err != domain.ErrReadingNotFound {
		t.Errorf("expected ErrReadingNotFound, got %v", err)
	}
}

func TestGetReadingsInRange(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	before := now.Add(-2 * time.Hour)
	within := now.Add(-1 * time.Hour)
	after := now.Add(1 * time.Hour)

	makeReading := func(lux float64, ts time.Time) *domain.LightReading {
		r, _ := domain.NewLightReading(lux)
		r.Timestamp = ts
		return r
	}

	_ = repo.SaveReading(ctx, makeReading(100, before))
	inRange := makeReading(200, within)
	_ = repo.SaveReading(ctx, inRange)
	_ = repo.SaveReading(ctx, makeReading(300, after))

	// Range: [now-90m, now) — only the within reading should appear
	start := now.Add(-90 * time.Minute)
	end := now

	results, err := repo.GetReadingsInRange(ctx, start, end)
	if err != nil {
		t.Fatalf("GetReadingsInRange failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 reading, got %d", len(results))
	}
	if results[0].Lux != 200 {
		t.Errorf("expected lux 200, got %v", results[0].Lux)
	}
}

func TestGetReadingsInRange_InclusiveStart(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	ts := time.Now().UTC().Truncate(time.Second)
	r, _ := domain.NewLightReading(100)
	r.Timestamp = ts
	_ = repo.SaveReading(ctx, r)

	// start == timestamp: should be included (inclusive start)
	results, err := repo.GetReadingsInRange(ctx, ts, ts.Add(time.Second))
	if err != nil {
		t.Fatalf("GetReadingsInRange failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result (inclusive start), got %d", len(results))
	}
}

func TestGetReadingsInRange_ExclusiveEnd(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	ts := time.Now().UTC().Truncate(time.Second)
	r, _ := domain.NewLightReading(100)
	r.Timestamp = ts
	_ = repo.SaveReading(ctx, r)

	// end == timestamp: should be excluded (exclusive end)
	results, err := repo.GetReadingsInRange(ctx, ts.Add(-time.Second), ts)
	if err != nil {
		t.Fatalf("GetReadingsInRange failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results (exclusive end), got %d", len(results))
	}
}

func TestDeleteOldReadings(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)

	makeReading := func(lux float64, ts time.Time) *domain.LightReading {
		r, _ := domain.NewLightReading(lux)
		r.Timestamp = ts
		return r
	}

	old := makeReading(100, now.Add(-48*time.Hour))
	recent := makeReading(200, now.Add(-1*time.Hour))
	_ = repo.SaveReading(ctx, old)
	_ = repo.SaveReading(ctx, recent)

	if err := repo.DeleteOldReadings(ctx, 24*time.Hour); err != nil {
		t.Fatalf("DeleteOldReadings failed: %v", err)
	}

	// Old reading should be gone
	_, err := repo.GetReading(ctx, old.ID)
	if err != domain.ErrReadingNotFound {
		t.Errorf("expected old reading to be deleted, got err: %v", err)
	}

	// Recent reading should remain
	_, err = repo.GetReading(ctx, recent.ID)
	if err != nil {
		t.Errorf("expected recent reading to remain, got err: %v", err)
	}
}
