package weather

import "fmt"

// Conditions holds structured current weather data for a location.
type Conditions struct {
	Location           string
	TemperatureCelsius float64
	Description        string
	Humidity           *int
}

// Summary returns a compact human-readable conditions string.
func (c *Conditions) Summary() string {
	description := c.Description
	if description == "" {
		description = "Unknown conditions"
	}

	temp := fmt.Sprintf("%.0f°C", c.TemperatureCelsius)
	if c.Humidity != nil {
		return fmt.Sprintf("%s, %s, Humidity: %d%%", temp, description, *c.Humidity)
	}
	return fmt.Sprintf("%s, %s", temp, description)
}

// Format returns a user-facing weather message for the given location label.
func (c *Conditions) Format(location string) string {
	label := location
	if c.Location != "" {
		label = c.Location
	}
	return fmt.Sprintf("Weather in %s: %s", label, c.Summary())
}
