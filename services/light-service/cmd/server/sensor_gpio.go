//go:build linux && arm64

package main

import (
	"github.com/quentinrf/plant-monitor/services/light-service/internal/adapters/gpio"
	"github.com/quentinrf/plant-monitor/services/light-service/internal/adapters/mock"
	"github.com/quentinrf/plant-monitor/services/light-service/internal/ports"
)

// newSensor returns the appropriate LightSensor for the current platform.
// On linux/arm64 (Raspberry Pi), SENSOR_TYPE=gpio uses the real BH1750 sensor;
// any other value falls back to the mock sensor.
func newSensor(sensorType string) (ports.LightSensor, error) {
	if sensorType == "gpio" {
		return gpio.NewBH1750Sensor()
	}
	return mock.NewFakeSensor(500.0, 100.0), nil
}
