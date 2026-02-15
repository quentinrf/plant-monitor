package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/quentinrf/plant-monitor/services/light-service/internal/domain"
)

// ReadingRepository implements domain.ReadingRepository with in-memory storage
// This is perfect for development - no database setup needed
type ReadingRepository struct {
	mu       sync.RWMutex
	readings map[int64]*domain.LightReading
	nextID   int64
}

// NewReadingRepository creates an empty in-memory repository
func NewReadingRepository() *ReadingRepository {
	return &ReadingRepository{
		readings: make(map[int64]*domain.LightReading),
		nextID:   1,
	}
}

// SaveReading stores a reading in memory
func (r *ReadingRepository) SaveReading(ctx context.Context, reading *domain.LightReading) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Assign ID if not set
	if reading.ID == 0 {
		reading.ID = r.nextID
		r.nextID++
	}

	// Store
	r.readings[reading.ID] = reading
	return nil
}

// GetReading retrieves a reading by ID
func (r *ReadingRepository) GetReading(ctx context.Context, id int64) (*domain.LightReading, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	reading, exists := r.readings[id]
	if !exists {
		return nil, domain.ErrReadingNotFound
	}

	return reading, nil
}

// GetReadingsInRange returns all readings within time range
func (r *ReadingRepository) GetReadingsInRange(ctx context.Context, start, end time.Time) ([]*domain.LightReading, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []*domain.LightReading
	for _, reading := range r.readings {
		if reading.Timestamp.After(start) && reading.Timestamp.Before(end) {
			results = append(results, reading)
		}
	}

	// Sort by timestamp
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.Before(results[j].Timestamp)
	})

	return results, nil
}

// GetLatestReading returns the most recent reading
func (r *ReadingRepository) GetLatestReading(ctx context.Context) (*domain.LightReading, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.readings) == 0 {
		return nil, domain.ErrReadingNotFound
	}

	var latest *domain.LightReading
	for _, reading := range r.readings {
		if latest == nil || reading.Timestamp.After(latest.Timestamp) {
			latest = reading
		}
	}

	return latest, nil
}

// DeleteOldReadings removes readings older than specified duration
func (r *ReadingRepository) DeleteOldReadings(ctx context.Context, olderThan time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)

	for id, reading := range r.readings {
		if reading.Timestamp.Before(cutoff) {
			delete(r.readings, id)
		}
	}

	return nil
}
