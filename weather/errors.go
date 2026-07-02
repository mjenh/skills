package weather

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrMissingAPIKey is returned when NewClient is called with an empty API key.
	ErrMissingAPIKey = errors.New("weather: API key is required")

	// ErrEmptyLocation is returned when GetConditions is called with an empty location.
	ErrEmptyLocation = errors.New("weather: location is required")
)

// GeocodeError indicates that a location could not be resolved to coordinates.
type GeocodeError struct {
	Location string
	Cause    error
}

func (e *GeocodeError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("weather: geocode %q: %v", e.Location, e.Cause)
	}
	return fmt.Sprintf("weather: geocode %q failed", e.Location)
}

func (e *GeocodeError) Unwrap() error {
	return e.Cause
}

// CoverageAreaError indicates the Weather API does not cover the requested region.
type CoverageAreaError struct {
	Location string
	Message  string
}

func (e *CoverageAreaError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("weather: coverage area for %q: %s", e.Location, e.Message)
	}
	return fmt.Sprintf("weather: coverage area not available for %q", e.Location)
}

// IsCoverageAreaError reports whether err is or wraps a CoverageAreaError.
func IsCoverageAreaError(err error) bool {
	var target *CoverageAreaError
	return errors.As(err, &target)
}

func newCoverageAreaError(location, message string) *CoverageAreaError {
	return &CoverageAreaError{
		Location: location,
		Message:  message,
	}
}

func isCoverageAreaMessage(message string) bool {
	msg := strings.ToLower(message)
	return strings.Contains(msg, "not supported") ||
		strings.Contains(msg, "not available") ||
		strings.Contains(msg, "restricted") ||
		strings.Contains(msg, "permission denied")
}
