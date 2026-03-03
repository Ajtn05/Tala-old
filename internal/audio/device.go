package audio

import (
	"fmt"
	"strings"

	"github.com/gordonklaus/portaudio"
)

func FindMicrophone() (*portaudio.DeviceInfo, error) {
	devices, err := portaudio.Devices()
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}

	var fallback *portaudio.DeviceInfo
	for _, d := range devices {
		if d.MaxInputChannels == 0 {
			continue
		}
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
	return nil, fmt.Errorf("microphone not found — no input device with \"microphone\" in its name")
}
