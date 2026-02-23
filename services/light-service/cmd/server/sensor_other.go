//go:build !linux || !arm64

package main

import (
	"fmt"

	"github.com/quentinrf/plant-monitor/services/light-service/internal/adapters/mock"
	"github.com/quentinrf/plant-monitor/services/light-service/internal/ports"
)

// newSensor returns the appropriate LightSensor for the current platform.
// On non-linux/arm64 platforms, only the mock sensor is available.
func newSensor(sensorType string) (ports.LightSensor, error) {
	if sensorType == "gpio" {
		return nil, fmt.Errorf("gpio sensor is only available on linux/arm64 (Raspberry Pi); set SENSOR_TYPE=mock")
	}
	return mock.NewFakeSensor(500.0, 100.0), nil
}
