package audio

import (
	"fmt"
	"strings"

	"github.com/gordonklaus/portaudio"
)

const (
	errAggregateNotFound  = "aggregate device not found — create one in Audio MIDI Setup combining BlackHole 2ch + Built-in Microphone"
	errBlackholeNotFound  = "blackhole device not found — ensure blackhole-2ch is installed"
	errMicrophoneNotFound = "microphone not found — no input device with \"microphone\" in its name"
)

func inputDevices() ([]*portaudio.DeviceInfo, error) {
	devices, err := portaudio.Devices()
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}

	var inputs []*portaudio.DeviceInfo
	for _, d := range devices {
		if d.MaxInputChannels > 0 {
			inputs = append(inputs, d)
		}
	}
	return inputs, nil
}

func findByName(keyword, errMsg string) (*portaudio.DeviceInfo, error) {
	devices, err := inputDevices()
	if err != nil {
		return nil, err
	}

	for _, d := range devices {
		if strings.Contains(strings.ToLower(d.Name), keyword) {
			return d, nil
		}
	}
	return nil, fmt.Errorf("%s", errMsg)
}

func FindDeviceForMode(mode Mode) (*portaudio.DeviceInfo, error) {
	switch mode {
	case ModeBoth:
		dev, err := findByName("kian aggregate", "")
		if err == nil {
			return dev, nil
		}
		return findByName("aggregate", errAggregateNotFound)
	case ModeMicrophone:
		return findBuiltInMicrophone()
	case ModeSystem:
		return findByName("blackhole", errBlackholeNotFound)
	default:
		return nil, fmt.Errorf("unknown audio mode %d", mode)
	}
}

func findBuiltInMicrophone() (*portaudio.DeviceInfo, error) {
	devices, err := inputDevices()
	if err != nil {
		return nil, err
	}

	var fallback *portaudio.DeviceInfo
	for _, d := range devices {
		name := strings.ToLower(d.Name)
		if !strings.Contains(name, "microphone") {
			continue
		}
		if strings.Contains(name, "built-in") {
			return d, nil
		}
		if fallback == nil {
			fallback = d
		}
	}
	if fallback != nil {
		return fallback, nil
	}
	return nil, fmt.Errorf("%s", errMicrophoneNotFound)
}
