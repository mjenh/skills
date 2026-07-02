package weather

import (
	"net/http"
	"time"
)

const (
	defaultGeocodeBaseURL = "https://maps.googleapis.com/maps/api/geocode/json"
	defaultWeatherBaseURL = "https://weather.googleapis.com/v1/currentConditions:lookup"
	defaultHTTPTimeout    = 10 * time.Second
)

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient sets the HTTP client used for API requests.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

// WithGeocodeBaseURL overrides the Google Geocoding API base URL.
func WithGeocodeBaseURL(baseURL string) Option {
	return func(c *Client) {
		if baseURL != "" {
			c.geocodeBaseURL = baseURL
		}
	}
}

// WithWeatherBaseURL overrides the Google Weather API base URL.
func WithWeatherBaseURL(baseURL string) Option {
	return func(c *Client) {
		if baseURL != "" {
			c.weatherBaseURL = baseURL
		}
	}
}
