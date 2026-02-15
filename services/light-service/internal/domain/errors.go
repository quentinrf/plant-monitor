package domain

import "errors"

var (
	// ErrInvalidLux indicates lux value is invalid
	ErrInvalidLux = errors.New("lux value cannot be negative")

	// ErrReadingNotFound indicates requested reading doesn't exist
	ErrReadingNotFound = errors.New("reading not found")

	// ErrSensorUnavailable indicates sensor cannot be read
	ErrSensorUnavailable = errors.New("sensor unavailable")
)
