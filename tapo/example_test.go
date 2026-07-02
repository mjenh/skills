package tapo_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/mjenh/skills/tapo"
)

func TestExample(t *testing.T) {
	host := os.Getenv("TAPO_HOST")
	if host == "" {
		host = os.Getenv("TAPO_IP")
	}
	email := os.Getenv("TAPO_EMAIL")
	password := os.Getenv("TAPO_PASSWORD")
	if host == "" || email == "" || password == "" {
		t.Skip("TAPO_HOST (or TAPO_IP), TAPO_EMAIL, and TAPO_PASSWORD required for integration example")
	}

	ctx := context.Background()

	plug, err := tapo.NewPlug(ctx, host, email, password)
	if err != nil {
		t.Fatal(err)
	}

	if err := plug.Toggle(ctx); err != nil {
		t.Fatal(err)
	}

	info, err := plug.DeviceInfo(ctx)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("Device: %s (on=%t)\n", info.Nickname, info.DeviceOn)
}
