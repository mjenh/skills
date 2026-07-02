# Changelog

All notable changes to [github.com/mjenh/skills/weather](https://github.com/mjenh/skills/weather) are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-07-02

### Added

- Pure Go `weather` package with `Client`, `NewClient`, and functional options
- `GetConditions` returning structured `Conditions` (`Location`, `TemperatureCelsius`, `Description`, `Humidity`)
- `GetWeather` returning a formatted user-facing string
- Google Maps Geocoding integration (`geocode.go`)
- Google Weather API current-conditions integration (`fetch.go`)
- Typed errors: `ErrMissingAPIKey`, `ErrEmptyLocation`, `GeocodeError`, `CoverageAreaError`
- `IsCoverageAreaError` helper for regulatory/coverage-area failures
- Client options: `WithHTTPClient`, `WithGeocodeBaseURL`, `WithWeatherBaseURL` (for tests and mocks)
- `weather/adk` subpackage with `New`, `NewFromEnv`, and `NewWithClient` for Google ADK `get_weather` FunctionTool
- `cmd/weather` CLI for manual lookups
- Co-located agent/MCP integration docs in `skill/SKILL.md` and `skill/tool.schema.json`
- `.env.example` documenting `WEATHER_API_KEY` and optional `LOG_LEVEL`

### Requirements

- Go 1.25.0 or later
- Google Maps API key with Weather API and Geocoding API enabled

[1.0.0]: https://github.com/mjenh/skills/releases/tag/weather/v1.0.0
