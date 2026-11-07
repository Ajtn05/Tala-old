package audio

import "errors"

// Linux microphone capture uses PortAudio. CoreAudio process taps are macOS-only.
type TapDevice struct{}

func CreateTapDevice() (*TapDevice, error) {
	return nil, errors.New("system audio capture is available on macOS only; on Linux record a monitor source with your audio tools and transcribe the resulting file")
}
func (*TapDevice) StartIO() error            { return errors.New("CoreAudio taps are unavailable on Linux") }
func (*TapDevice) ReadSamples([]float32) int { return 0 }
func (*TapDevice) Channels() int             { return 0 }
func (*TapDevice) StopIO()                   {}
func (*TapDevice) Destroy() error            { return nil }
