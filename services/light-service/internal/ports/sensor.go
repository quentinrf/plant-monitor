package ports

import (
	"context"
)

// LightSensor defines how to read light levels
// This is a PORT - adapters (GPIO, Mock) will implement it
type LightSensor interface {
	// ReadLux returns current light level in lux
	ReadLux(ctx context.Context) (float64, error)
	
	// Close releases any resources
	Close() error
}