package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/mjenh/skills/tapo"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: tapo <on|off|toggle|info>")
		os.Exit(1)
	}

	ctx := context.Background()
	plug, err := tapo.NewPlugFromEnv(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "on":
		err = plug.TurnOn(ctx)
	case "off":
		err = plug.TurnOff(ctx)
	case "toggle":
		err = plug.Toggle(ctx)
	case "info":
		info, infoErr := plug.DeviceInfo(ctx)
		if info != nil {
			fmt.Printf("nickname=%s model=%s on=%t ip=%s mac=%s\n",
				info.Nickname, info.Model, info.DeviceOn, info.IPAddress, info.MAC)
		}
		if infoErr != nil && !errors.Is(infoErr, tapo.ErrUnsupportedModel) {
			fmt.Fprintln(os.Stderr, infoErr)
			os.Exit(1)
		}
		if errors.Is(infoErr, tapo.ErrUnsupportedModel) {
			fmt.Fprintln(os.Stderr, "warning:", infoErr)
		}
		return
	default:
		fmt.Fprintln(os.Stderr, "usage: tapo <on|off|toggle|info>")
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
