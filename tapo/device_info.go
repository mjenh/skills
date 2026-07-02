package tapo

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

// DeviceInfo contains the device state and metadata returned by a Tapo plug.
// Base64-encoded fields (Nickname, SSID) are automatically decoded to plain
// UTF-8 strings.
type DeviceInfo struct {
	DeviceOn        bool   `json:"device_on"`
	Model           string `json:"model"`
	Nickname        string `json:"nickname"`
	DeviceID        string `json:"device_id"`
	FirmwareVersion string `json:"fw_ver"`
	HardwareVersion string `json:"hw_ver"`
	IPAddress       string `json:"ip"`
	MAC             string `json:"mac"`
	SSID            string `json:"ssid"`
}

// deviceInfoWire mirrors DeviceInfo but keeps base64-encoded fields as raw
// strings for two-pass decoding.
type deviceInfoWire struct {
	DeviceOn        bool   `json:"device_on"`
	Model           string `json:"model"`
	Nickname        string `json:"nickname"`
	DeviceID        string `json:"device_id"`
	FirmwareVersion string `json:"fw_ver"`
	HardwareVersion string `json:"hw_ver"`
	IPAddress       string `json:"ip"`
	MAC             string `json:"mac"`
	SSID            string `json:"ssid"`
}

// decodeBase64 attempts to decode a base64 standard-encoded string.
// On failure, the original value is returned unchanged.
func decodeBase64(s string) string {
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil || !utf8.Valid(decoded) {
		return s
	}
	return string(decoded)
}

// parseDeviceInfo deserializes the JSON response from get_device_info into
// a DeviceInfo struct, decoding base64 fields (Nickname, SSID) in one pass
// per AD-8. Returns a populated *DeviceInfo even when the model is not P100
// (in that case, the returned error wraps ErrUnsupportedModel per FR-8).
func parseDeviceInfo(data []byte) (*DeviceInfo, error) {
	var wire deviceInfoWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("tapo: failed to parse device info: %w", err)
	}

	info := &DeviceInfo{
		DeviceOn:        wire.DeviceOn,
		Model:           wire.Model,
		Nickname:        decodeBase64(wire.Nickname),
		DeviceID:        wire.DeviceID,
		FirmwareVersion: wire.FirmwareVersion,
		HardwareVersion: wire.HardwareVersion,
		IPAddress:       wire.IPAddress,
		MAC:             wire.MAC,
		SSID:            decodeBase64(wire.SSID),
	}

	if info.Model != "P100" {
		return info, fmt.Errorf("tapo: device model %q is not P100: %w", info.Model, ErrUnsupportedModel)
	}

	return info, nil
}
