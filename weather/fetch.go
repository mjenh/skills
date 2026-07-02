package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type weatherAPIResponse struct {
	Temperature      weatherTemp      `json:"temperature"`
	WeatherCondition weatherCondition `json:"weatherCondition"`
	RelativeHumidity *int             `json:"relativeHumidity"`
}

type weatherAPIError struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

type weatherTemp struct {
	Degrees float64 `json:"degrees"`
}

type weatherCondition struct {
	Description localizedText `json:"description"`
}

type localizedText struct {
	Text string `json:"text"`
}

func (c *Client) fetchConditions(ctx context.Context, location string, lat, lng float64) (*Conditions, error) {
	endpoint := fmt.Sprintf("%s?key=%s&location.latitude=%.6f&location.longitude=%.6f",
		c.weatherBaseURL, url.QueryEscape(c.apiKey), lat, lng)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr weatherAPIError
		if json.Unmarshal(body, &apiErr) == nil && apiErr.Error.Message != "" {
			if isCoverageAreaMessage(apiErr.Error.Message) {
				return nil, newCoverageAreaError(location, apiErr.Error.Message)
			}
			return nil, fmt.Errorf("weather: %s (HTTP %d)", apiErr.Error.Message, resp.StatusCode)
		}
		return nil, fmt.Errorf("weather: HTTP %d", resp.StatusCode)
	}

	var parsed weatherAPIResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}

	return &Conditions{
		TemperatureCelsius: parsed.Temperature.Degrees,
		Description:        parsed.WeatherCondition.Description.Text,
		Humidity:           parsed.RelativeHumidity,
	}, nil
}
