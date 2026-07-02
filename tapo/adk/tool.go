// Package adk adapts the tapo module for Google ADK agents.
package adk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/mjenh/skills/tapo"
)

type tapoPlugArgs struct {
	Action string `json:"action"`
}

type tapoPlugResult struct {
	Action   string          `json:"action"`
	Message  string          `json:"message"`
	Device   *tapoDeviceInfo `json:"device,omitempty"`
	Warning  string          `json:"warning,omitempty"`
}

type tapoDeviceInfo struct {
	DeviceOn        bool   `json:"deviceOn"`
	Model           string `json:"model"`
	Nickname        string `json:"nickname"`
	DeviceID        string `json:"deviceId"`
	FirmwareVersion string `json:"firmwareVersion"`
	HardwareVersion string `json:"hardwareVersion"`
	IPAddress       string `json:"ipAddress"`
	MAC             string `json:"mac"`
	SSID            string `json:"ssid"`
}

// New returns a tapo_plug FunctionTool using explicit credentials.
func New(host, email, password string) (tool.Tool, error) {
	plug, err := tapo.NewPlug(context.Background(), host, email, password)
	if err != nil {
		return nil, err
	}
	return NewWithPlug(plug)
}

// NewFromEnv returns a tapo_plug FunctionTool using TAPO_HOST, TAPO_EMAIL, and TAPO_PASSWORD.
func NewFromEnv() (tool.Tool, error) {
	plug, err := tapo.NewPlugFromEnv(context.Background())
	if err != nil {
		return nil, err
	}
	return NewWithPlug(plug)
}

// NewWithPlug returns a tapo_plug FunctionTool backed by an existing plug client.
func NewWithPlug(plug *tapo.Plug) (tool.Tool, error) {
	if plug == nil {
		return nil, fmt.Errorf("adk: tapo plug is required")
	}

	return functiontool.New(functiontool.Config{
		Name:        "tapo_plug",
		Description: "Control a Tapo P100 smart plug on the local network. Actions: on, off, toggle, info.",
	}, func(_ agent.ToolContext, args tapoPlugArgs) (string, error) {
		return handleTapoPlug(context.Background(), plug, args)
	})
}

func handleTapoPlug(ctx context.Context, plug *tapo.Plug, args tapoPlugArgs) (string, error) {
	action := strings.ToLower(strings.TrimSpace(args.Action))
	if action == "" {
		return "Please provide an action: on, off, toggle, or info.", nil
	}

	result := tapoPlugResult{Action: action}

	switch action {
	case "on":
		err := plug.TurnOn(ctx)
		result.Message = "Tapo plug turned on."
		return encodeResult(result, err)
	case "off":
		err := plug.TurnOff(ctx)
		result.Message = "Tapo plug turned off."
		return encodeResult(result, err)
	case "toggle":
		err := plug.Toggle(ctx)
		result.Message = "Tapo plug toggled."
		return encodeResult(result, err)
	case "info":
		info, err := plug.DeviceInfo(ctx)
		if info != nil {
			result.Device = toTapoDeviceInfo(info)
			result.Message = fmt.Sprintf("Device %s is on=%t.", info.Nickname, info.DeviceOn)
		}
		if errors.Is(err, tapo.ErrUnsupportedModel) {
			result.Warning = err.Error()
			err = nil
		}
		return encodeResult(result, err)
	default:
		return fmt.Sprintf("Unknown action %q. Use on, off, toggle, or info.", action), nil
	}
}

func encodeResult(result tapoPlugResult, err error) (string, error) {
	if err != nil && errors.Is(err, tapo.ErrUnsupportedModel) {
		result.Warning = err.Error()
		err = nil
	}

	if err != nil {
		switch {
		case errors.Is(err, tapo.ErrAuth):
			slog.Error("tapo auth failed", "err", err)
			return "Authentication failed. Check TAPO_EMAIL and TAPO_PASSWORD.", nil
		case errors.Is(err, tapo.ErrTimeout):
			slog.Error("tapo timeout", "err", err)
			return "The Tapo device did not respond in time. Check TAPO_HOST and network connectivity.", nil
		case errors.Is(err, tapo.ErrHandshake):
			slog.Error("tapo handshake failed", "err", err)
			return "Could not establish a transport session with the Tapo device.", nil
		default:
			slog.Error("tapo command failed", "action", result.Action, "err", err)
			return fmt.Sprintf("Tapo %s failed: %v", result.Action, err), nil
		}
	}

	data, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return result.Message, nil
	}
	return string(data), nil
}

func toTapoDeviceInfo(info *tapo.DeviceInfo) *tapoDeviceInfo {
	return &tapoDeviceInfo{
		DeviceOn:        info.DeviceOn,
		Model:           info.Model,
		Nickname:        info.Nickname,
		DeviceID:        info.DeviceID,
		FirmwareVersion: info.FirmwareVersion,
		HardwareVersion: info.HardwareVersion,
		IPAddress:       info.IPAddress,
		MAC:             info.MAC,
		SSID:            info.SSID,
	}
}
