package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/quentinrf/plant-monitor/services/light-service/internal/domain"
)

// ReadingRepository implements domain.ReadingRepository with SQLite
type ReadingRepository struct {
	db *sql.DB
}

// NewReadingRepository creates a SQLite-backed repository
func NewReadingRepository(dbPath string) (*ReadingRepository, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create table if not exists
	schema := `
	CREATE TABLE IF NOT EXISTS light_readings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		lux REAL NOT NULL,
		timestamp DATETIME NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_timestamp ON light_readings(timestamp);
	`

	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	return &ReadingRepository{db: db}, nil
}

// SaveReading stores a reading in SQLite
func (r *ReadingRepository) SaveReading(ctx context.Context, reading *domain.LightReading) error {
	query := `INSERT INTO light_readings (lux, timestamp) VALUES (?, ?)`

	result, err := r.db.ExecContext(ctx, query, reading.Lux, reading.Timestamp)
	if err != nil {
		return fmt.Errorf("failed to insert reading: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get insert id: %w", err)
	}

	reading.ID = id
	return nil
}

// GetReading retrieves a reading by ID
func (r *ReadingRepository) GetReading(ctx context.Context, id int64) (*domain.LightReading, error) {
	query := `SELECT id, lux, timestamp FROM light_readings WHERE id = ?`

	var reading domain.LightReading
	var timestamp string

	err := r.db.QueryRowContext(ctx, query, id).Scan(&reading.ID, &reading.Lux, &timestamp)
	if err == sql.ErrNoRows {
		return nil, domain.ErrReadingNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query reading: %w", err)
	}

	reading.Timestamp, err = time.Parse("2006-01-02 15:04:05", timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	return &reading, nil
}

// GetReadingsInRange returns all readings within time range
func (r *ReadingRepository) GetReadingsInRange(ctx context.Context, start, end time.Time) ([]*domain.LightReading, error) {
	query := `
		SELECT id, lux, timestamp 
		FROM light_readings 
		WHERE timestamp >= ? AND timestamp <= ?
		ORDER BY timestamp ASC
	`

	rows, err := r.db.QueryContext(ctx, query, start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"))
	if err != nil {
		return nil, fmt.Errorf("failed to query readings: %w", err)
	}
	defer rows.Close()

	var readings []*domain.LightReading
	for rows.Next() {
		var reading domain.LightReading
		var timestamp string

		if err := rows.Scan(&reading.ID, &reading.Lux, &timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan reading: %w", err)
		}

		reading.Timestamp, err = time.Parse("2006-01-02 15:04:05", timestamp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse timestamp: %w", err)
		}

		readings = append(readings, &reading)
	}

	return readings, nil
}

// GetLatestReading returns the most recent reading
func (r *ReadingRepository) GetLatestReading(ctx context.Context) (*domain.LightReading, error) {
	query := `
		SELECT id, lux, timestamp 
		FROM light_readings 
		ORDER BY timestamp DESC 
		LIMIT 1
	`

	var reading domain.LightReading
	var timestamp string

	err := r.db.QueryRowContext(ctx, query).Scan(&reading.ID, &reading.Lux, &timestamp)
	if err == sql.ErrNoRows {
		return nil, domain.ErrReadingNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query latest reading: %w", err)
	}

	reading.Timestamp, err = time.Parse("2006-01-02 15:04:05", timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	return &reading, nil
}

// DeleteOldReadings removes readings older than specified duration
func (r *ReadingRepository) DeleteOldReadings(ctx context.Context, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	query := `DELETE FROM light_readings WHERE timestamp < ?`

	_, err := r.db.ExecContext(ctx, query, cutoff.Format("2006-01-02 15:04:05"))
	if err != nil {
		return fmt.Errorf("failed to delete old readings: %w", err)
	}

	return nil
}

// Close closes the database connection
func (r *ReadingRepository) Close() error {
	return r.db.Close()
}
