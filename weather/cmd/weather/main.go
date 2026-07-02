package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mjenh/skills/weather"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: weather <location>")
		os.Exit(1)
	}

	client, err := weather.NewClient(os.Getenv("WEATHER_API_KEY"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	location := os.Args[1]
	result, err := client.GetWeather(context.Background(), location)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println(result)
}
