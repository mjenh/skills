# weather

Pure Go module for current weather conditions by location name, backed by the Google Weather API and Google Maps Geocoding API.

## Install

```bash
go get github.com/mjenh/skills/weather
```

## Setup

Copy `.env.example` to `.env` and set your Google Maps API key:

```bash
export WEATHER_API_KEY=your-google-maps-api-key
```

The key must have **Weather API** and **Geocoding API** enabled.

## Usage

```go
package example

import (
    "context"

    "github.com/mjenh/skills/weather"
)

func Example(apiKey, location string) (string, error) {
    client, err := weather.NewClient(apiKey)
    if err != nil {
        return "", err
    }

    conditions, err := client.GetConditions(context.Background(), location)
    if err != nil {
        return "", err
    }

    return conditions.Format(location), nil
}
```

### Structured result

`GetConditions` returns:

| Field | Type | Description |
|-------|------|-------------|
| `Location` | `string` | Resolved input location |
| `TemperatureCelsius` | `float64` | Current temperature |
| `Description` | `string` | Weather condition text |
| `Humidity` | `*int` | Relative humidity when available |

### Options

```go
weather.NewClient(apiKey,
    weather.WithHTTPClient(customHTTPClient),
    weather.WithGeocodeBaseURL(testGeocodeURL),
    weather.WithWeatherBaseURL(testWeatherURL),
)
```

Base URL overrides are intended for tests and local mocks.

## CLI

```bash
go run ./cmd/weather "San Francisco"
```

## Google ADK

```go
import weatheradk "github.com/mjenh/skills/weather/adk"

tool, err := weatheradk.NewFromEnv()
```

## MCP / agent integration

See [`skill/SKILL.md`](skill/SKILL.md) and [`skill/tool.schema.json`](skill/tool.schema.json).

## Layout

```
weather/          package weather — pure Go, no ADK/MCP imports
adk/              Google ADK FunctionTool adapter
cmd/weather/      CLI for manual testing
skill/            Agent/MCP integration docs and tool schema
```
