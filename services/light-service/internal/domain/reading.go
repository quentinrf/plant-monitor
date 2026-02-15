package domain

import (
	"time"
)

// LightReading represents a single light measurement
// This is pure domain logic - no database, no gRPC, just business concepts
type LightReading struct {
	ID        int64
	Lux       float64
	Timestamp time.Time
}

// NewLightReading creates a new reading with validation
func NewLightReading(lux float64) (*LightReading, error) {
	// Business rule: Lux cannot be negative
	if lux < 0 {
		return nil, ErrInvalidLux
	}

	return &LightReading{
		Lux:       lux,
		Timestamp: time.Now(),
	}, nil
}

// IsLowLight returns true if reading indicates low light conditions
// Business logic: < 200 lux is considered low light
func (r *LightReading) IsLowLight() bool {
	return r.Lux < 200
}

// IsMediumLight returns true if reading indicates medium light
// Business logic: 200-2500 lux is medium light
func (r *LightReading) IsMediumLight() bool {
	return r.Lux >= 200 && r.Lux < 2500
}

// IsHighLight returns true if reading indicates high light
// Business logic: >= 2500 lux is high light
func (r *LightReading) IsHighLight() bool {
	return r.Lux >= 2500
}

// LightCategory returns human-readable category
func (r *LightReading) LightCategory() string {
	if r.IsLowLight() {
		return "Low Light"
	} else if r.IsMediumLight() {
		return "Medium Light"
	}
	return "High Light"
}
