package ports

import (
	"context"
	"time"
)

// LightReading is a simplified reading as seen by plant-service.
// The full reading model lives in light-service; this type carries only what
// plant-service needs, decoupling the two services at the domain boundary.
type LightReading struct {
	Lux       float64
	Timestamp time.Time
	Category  string
}

// LightClient is the port for fetching light data from light-service.
// Implementations: adapters/grpc.LightClientAdapter (production),
// mock.LightClient (tests).
type LightClient interface {
	// GetCurrentLux returns the most recent light reading from light-service.
	GetCurrentLux(ctx context.Context) (*LightReading, error)

	// GetHistory returns all readings in [start, end).
	GetHistory(ctx context.Context, start, end time.Time) ([]LightReading, error)

	// Close releases the underlying connection.
	Close() error
}
