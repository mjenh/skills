// Package adk adapts the weather module for Google ADK agents.
package adk

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/mjenh/skills/weather"
)

type weatherArgs struct {
	Location string `json:"location"`
}

// New returns a get_weather FunctionTool using the given API key.
func New(apiKey string) (tool.Tool, error) {
	client, err := weather.NewClient(apiKey)
	if err != nil {
		return nil, err
	}
	return NewWithClient(client)
}

// NewFromEnv returns a get_weather FunctionTool using WEATHER_API_KEY.
func NewFromEnv() (tool.Tool, error) {
	return New(os.Getenv("WEATHER_API_KEY"))
}

// NewWithClient returns a get_weather FunctionTool backed by an existing client.
func NewWithClient(client *weather.Client) (tool.Tool, error) {
	if client == nil {
		return nil, fmt.Errorf("adk: weather client is required")
	}

	return functiontool.New(functiontool.Config{
		Name:        "get_weather",
		Description: "Get current weather conditions for a location including temperature, conditions, and humidity.",
	}, func(_ agent.ToolContext, args weatherArgs) (string, error) {
		return handleGetWeather(context.Background(), client, args)
	})
}

func handleGetWeather(ctx context.Context, client *weather.Client, args weatherArgs) (string, error) {
	if args.Location == "" {
		return "Please provide a location to get weather for.", nil
	}

	conditions, err := client.GetConditions(ctx, args.Location)
	if err != nil {
		switch {
		case weather.IsCoverageAreaError(err):
			slog.Error("fetch weather failed", "location", args.Location, "err", err)
			return fmt.Sprintf(
				"Weather data is not available for %s. The Google Weather API does not cover this region due to regulatory restrictions.",
				args.Location,
			), nil
		default:
			slog.Error("get weather failed", "location", args.Location, "err", err)
			return fmt.Sprintf("Unable to retrieve weather for %s. Please try again.", args.Location), nil
		}
	}

	return conditions.Format(args.Location), nil
}
