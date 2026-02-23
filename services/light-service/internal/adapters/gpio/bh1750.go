//go:build linux && arm64

// Package gpio provides a BH1750 ambient light sensor adapter via periph.io.
//
// Before building for linux/arm64, add the required dependencies:
//
//	go get periph.io/x/host/v3@latest
//	go get periph.io/x/conn/v3@latest
//	go get periph.io/x/devices/v3@latest
//
// Wiring (GY-302 / BH1750 module → Raspberry Pi 40-pin header):
//
//	BH1750 VCC → Pin  1  (3.3 V)
//	BH1750 GND → Pin  6  (GND)
//	BH1750 SDA → Pin  3  (GPIO 2 / SDA1)
//	BH1750 SCL → Pin  5  (GPIO 3 / SCL1)
//
// Enable I²C on the Pi before use:
//
//	sudo raspi-config → Interface Options → I2C → Enable
//	Verify: i2cdetect -y 1  (should show device at address 0x23)
package gpio

import (
	"context"
	"fmt"

	"periph.io/x/conn/v3/i2c/i2creg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/devices/v3/bh1750"
	"periph.io/x/host/v3"
)

// BH1750Sensor implements ports.LightSensor using a BH1750 I²C light sensor.
type BH1750Sensor struct {
	dev *bh1750.Dev
}

// NewBH1750Sensor initialises periph.io host drivers and opens the sensor on
// the default I²C bus (/dev/i2c-1 on Raspberry Pi).
func NewBH1750Sensor() (*BH1750Sensor, error) {
	if _, err := host.Init(); err != nil {
		return nil, fmt.Errorf("periph.io host init: %w", err)
	}

	bus, err := i2creg.Open("")
	if err != nil {
		return nil, fmt.Errorf("open I2C bus: %w", err)
	}

	dev, err := bh1750.NewI2C(bus, &bh1750.DefaultOpts)
	if err != nil {
		return nil, fmt.Errorf("open BH1750: %w", err)
	}

	return &BH1750Sensor{dev: dev}, nil
}

// ReadLux returns the current illuminance in lux.
//
// physic.Illuminance is stored in nano-lux; dividing by physic.Lux converts to
// float64 lux suitable for the domain layer.
func (s *BH1750Sensor) ReadLux(_ context.Context) (float64, error) {
	lux, err := s.dev.SenseIlluminance()
	if err != nil {
		return 0, fmt.Errorf("bh1750 sense: %w", err)
	}
	return float64(lux) / float64(physic.Lux), nil
}

// Close halts the sensor and releases I²C resources.
func (s *BH1750Sensor) Close() error {
	return s.dev.Halt()
}
