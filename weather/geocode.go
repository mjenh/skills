package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type geocodeResponse struct {
	Results []geocodeResult `json:"results"`
	Status  string          `json:"status"`
}

type geocodeResult struct {
	Geometry geocodeGeometry `json:"geometry"`
}

type geocodeGeometry struct {
	Location geocodeLatLng `json:"location"`
}

type geocodeLatLng struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

func (c *Client) geocode(ctx context.Context, location string) (float64, float64, error) {
	endpoint := fmt.Sprintf("%s?address=%s&key=%s",
		c.geocodeBaseURL, url.QueryEscape(location), url.QueryEscape(c.apiKey))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, 0, &GeocodeError{Location: location, Cause: err}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, 0, &GeocodeError{Location: location, Cause: err}
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, &GeocodeError{Location: location, Cause: err}
	}

	if resp.StatusCode != http.StatusOK {
		return 0, 0, &GeocodeError{
			Location: location,
			Cause:    fmt.Errorf("HTTP %d", resp.StatusCode),
		}
	}

	var parsed geocodeResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, 0, &GeocodeError{Location: location, Cause: err}
	}

	if parsed.Status != "OK" || len(parsed.Results) == 0 {
		return 0, 0, &GeocodeError{
			Location: location,
			Cause:    fmt.Errorf("no results for %q", location),
		}
	}

	loc := parsed.Results[0].Geometry.Location
	return loc.Lat, loc.Lng, nil
}
