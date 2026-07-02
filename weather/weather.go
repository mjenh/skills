package weather

import (
	"context"
	"net/http"
)

// Client retrieves weather data from Google APIs.
type Client struct {
	apiKey         string
	httpClient     *http.Client
	geocodeBaseURL string
	weatherBaseURL string
}

// NewClient creates a weather client with the given Google Maps API key.
func NewClient(apiKey string, opts ...Option) (*Client, error) {
	if apiKey == "" {
		return nil, ErrMissingAPIKey
	}

	client := &Client{
		apiKey:         apiKey,
		httpClient:     &http.Client{Timeout: defaultHTTPTimeout},
		geocodeBaseURL: defaultGeocodeBaseURL,
		weatherBaseURL: defaultWeatherBaseURL,
	}
	for _, opt := range opts {
		opt(client)
	}
	return client, nil
}

// GetConditions geocodes the location and fetches current weather conditions.
func (c *Client) GetConditions(ctx context.Context, location string) (*Conditions, error) {
	if location == "" {
		return nil, ErrEmptyLocation
	}

	lat, lng, err := c.geocode(ctx, location)
	if err != nil {
		return nil, err
	}

	conditions, err := c.fetchConditions(ctx, location, lat, lng)
	if err != nil {
		return nil, err
	}

	conditions.Location = location
	return conditions, nil
}

// GetWeather returns a formatted weather message for the given location.
func (c *Client) GetWeather(ctx context.Context, location string) (string, error) {
	conditions, err := c.GetConditions(ctx, location)
	if err != nil {
		return "", err
	}
	return conditions.Format(location), nil
}
