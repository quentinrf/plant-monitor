package ports

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/quentinrf/plant-monitor/services/light-service/internal/domain"
)

// Recorder handles periodic sensor reading and storage
type Recorder struct {
	sensor   LightSensor
	repo     domain.ReadingRepository
	interval time.Duration
}

// NewRecorder creates a new background recorder
func NewRecorder(sensor LightSensor, repo domain.ReadingRepository, interval time.Duration) *Recorder {
	return &Recorder{
		sensor:   sensor,
		repo:     repo,
		interval: interval,
	}
}

// Start begins periodic sensor reading
// This runs in a goroutine until context is cancelled
func (r *Recorder) Start(ctx context.Context) {
	log.Info().
		Dur("interval", r.interval).
		Msg("starting background recorder")

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	cleanupTicker := time.NewTicker(24 * time.Hour)
	defer cleanupTicker.Stop()

	// Record immediately on start
	r.recordOnce(ctx)

	for {
		select {
		case <-ticker.C:
			r.recordOnce(ctx)

		case <-cleanupTicker.C:
			if err := r.repo.DeleteOldReadings(ctx, 30*24*time.Hour); err != nil {
				log.Error().Err(err).Msg("failed to delete old readings")
			} else {
				log.Info().Msg("deleted readings older than 30 days")
			}

		case <-ctx.Done():
			log.Info().Msg("stopping background recorder")
			return
		}
	}
}

// recordOnce reads sensor and saves to repository
func (r *Recorder) recordOnce(ctx context.Context) {
	log.Debug().Msg("reading sensor")

	lux, err := r.sensor.ReadLux(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to read sensor")
		return
	}

	reading, err := domain.NewLightReading(lux)
	if err != nil {
		log.Error().Err(err).Msg("failed to create reading")
		return
	}

	if err := r.repo.SaveReading(ctx, reading); err != nil {
		log.Error().Err(err).Msg("failed to save reading")
		return
	}

	log.Info().
		Float64("lux", lux).
		Str("category", reading.LightCategory()).
		Msg("recorded light reading")
}
