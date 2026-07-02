---
name: weather
description: >-
  Get current weather for a location via Google Weather and Geocoding APIs.
  Use when building MCP tools or agents that need live weather data, or when
  integrating github.com/mjenh/skills/weather into an agent runtime.
---

# Weather Module

Pure Go library for current weather conditions by location name.

## Go import

```go
import "github.com/mjenh/skills/weather"
```

## Environment

| Variable | Required | Description |
|----------|----------|-------------|
| `WEATHER_API_KEY` | Yes | Google Maps API key with Weather API and Geocoding API enabled |

See `../.env.example` for a template.

## Public API

```go
client, err := weather.NewClient(apiKey)
conditions, err := client.GetConditions(ctx, "Tokyo")
// conditions.TemperatureCelsius, conditions.Description, conditions.Humidity

message, err := client.GetWeather(ctx, "Tokyo")
```

## MCP integration

Expose tool `get_weather` using the schema in `tool.schema.json`:

1. Parse `{ "location": "string" }` from the tool call.
2. Call `client.GetConditions(ctx, location)`.
3. Return structured JSON from `Conditions` or a formatted string via `conditions.Format(location)`.

## Google ADK integration

Import the adapter subpackage instead of wiring ADK in your app:

```go
import weatheradk "github.com/mjenh/skills/weather/adk"

tool, err := weatheradk.NewFromEnv()
```

## Errors

| Error | Meaning |
|-------|---------|
| `weather.ErrMissingAPIKey` | Empty API key passed to `NewClient` |
| `weather.ErrEmptyLocation` | Empty location passed to `GetConditions` |
| `*weather.GeocodeError` | Location could not be geocoded |
| `*weather.CoverageAreaError` | Region not covered by Google Weather API |

Use `weather.IsCoverageAreaError(err)` to detect regulatory coverage failures.
