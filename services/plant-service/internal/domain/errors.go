package domain

import "errors"

var (
	// ErrNoHistory indicates no light history is available to analyze.
	ErrNoHistory = errors.New("no light history available")

	// ErrLightClientUnavailable indicates the upstream light-service cannot be reached.
	ErrLightClientUnavailable = errors.New("light client unavailable")
)
