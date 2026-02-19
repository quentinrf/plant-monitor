package domain

import (
	"context"
	"time"
)

// ReadingRepository defines operations for storing/retrieving readings
// This is a PORT - adapters (SQLite, Postgres, Memory) will implement it
type ReadingRepository interface {
	// SaveReading persists a reading
	SaveReading(ctx context.Context, reading *LightReading) error

	// GetReading retrieves a specific reading by ID
	GetReading(ctx context.Context, id int64) (*LightReading, error)

	// GetReadingsInRange retrieves all readings within time range.
	// Uses a half-open interval: inclusive start, exclusive end [start, end).
	GetReadingsInRange(ctx context.Context, start, end time.Time) ([]*LightReading, error)

	// GetLatestReading retrieves the most recent reading
	GetLatestReading(ctx context.Context) (*LightReading, error)

	// DeleteOldReadings removes readings older than specified duration
	// Business rule: We might want to retain only last 30 days
	DeleteOldReadings(ctx context.Context, olderThan time.Duration) error
}
