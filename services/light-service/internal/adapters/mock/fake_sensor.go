package mock

import (
	"context"
	"math/rand"
)

// FakeSensor simulates a light sensor for development
// This implements the ports.LightSensor interface
type FakeSensor struct {
	baseValue float64
	variation float64
}

// NewFakeSensor creates a sensor that returns realistic values
// baseValue: average lux (e.g., 500 for indoor lighting)
// variation: +/- range (e.g., 100 means 400-600)
func NewFakeSensor(baseValue, variation float64) *FakeSensor {
	return &FakeSensor{
		baseValue: baseValue,
		variation: variation,
	}
}

// ReadLux returns a simulated light reading
// Simulates realistic variance (lights flicker, clouds pass, etc.)
func (s *FakeSensor) ReadLux(ctx context.Context) (float64, error) {
	// Random value around base ± variation
	variance := (rand.Float64() - 0.5) * 2 * s.variation
	lux := s.baseValue + variance
	
	// Ensure non-negative
	if lux < 0 {
		lux = 0
	}
	
	return lux, nil
}

// Close is a no-op for fake sensor
func (s *FakeSensor) Close() error {
	return nil
}