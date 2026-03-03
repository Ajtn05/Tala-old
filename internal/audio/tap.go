package audio

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreAudio -framework CoreFoundation -framework Foundation

#include "tap_darwin.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

const (
	tapName = "Kian Tap"
	tapUID  = "com.kian.tap"
)

// TapDevice holds the CoreAudio resources for a process tap session.
type TapDevice struct {
	tapID       C.AudioObjectID
	aggregateID C.AudioDeviceID
}

// CreateTapDevice creates a CoreAudio process tap and wraps it in a private
// aggregate device. The aggregate is read via CoreAudio IO procs (not PortAudio)
// because private aggregates are required for tap input channels to appear.
func CreateTapDevice() (*TapDevice, error) {
	var tapID C.AudioObjectID
	status := C.createGlobalTap(&tapID)
	if status != 0 {
		return nil, fmt.Errorf("create process tap: CoreAudio error %d", int(status))
	}

	var uuidBuf [256]C.char
	status = C.getTapUUID(tapID, &uuidBuf[0], 256)
	if status != 0 {
		C.destroyTap(tapID)
		return nil, fmt.Errorf("get tap UUID: CoreAudio error %d", int(status))
	}
	tapUUID := C.GoString(&uuidBuf[0])

	cUUID := C.CString(tapUUID)
	defer C.free(unsafe.Pointer(cUUID))
	cName := C.CString(tapName)
	defer C.free(unsafe.Pointer(cName))
	cUID := C.CString(tapUID)
	defer C.free(unsafe.Pointer(cUID))

	var aggregateID C.AudioDeviceID
	status = C.createTapAggregate(cUUID, cName, cUID, &aggregateID)
	if status != 0 {
		C.destroyTap(tapID)
		return nil, fmt.Errorf("create tap aggregate device: CoreAudio error %d", int(status))
	}

	return &TapDevice{
		tapID:       tapID,
		aggregateID: aggregateID,
	}, nil
}

// StartIO begins reading audio from the tap via CoreAudio IO procs.
func (t *TapDevice) StartIO() error {
	status := C.startTapIO(t.aggregateID)
	if status != 0 {
		return fmt.Errorf("start tap IO: CoreAudio error %d", int(status))
	}
	return nil
}

// ReadSamples copies available samples from the tap ring buffer.
// Returns the number of samples copied (interleaved stereo).
func (t *TapDevice) ReadSamples(buf []float32) int {
	if len(buf) == 0 {
		return 0
	}
	return int(C.readTapSamples((*C.float)(unsafe.Pointer(&buf[0])), C.int(len(buf))))
}

// Channels returns the number of input channels on the tap aggregate.
func (t *TapDevice) Channels() int {
	return int(C.getTapChannels())
}

// StopIO stops the CoreAudio IO proc.
func (t *TapDevice) StopIO() {
	C.stopTapIO(t.aggregateID)
}

// Destroy releases the aggregate device and process tap.
func (t *TapDevice) Destroy() error {
	if t == nil {
		return nil
	}

	C.stopTapIO(t.aggregateID)

	var firstErr error
	if status := C.destroyAggregate(t.aggregateID); status != 0 {
		firstErr = fmt.Errorf("destroy aggregate device: CoreAudio error %d", int(status))
	}
	if status := C.destroyTap(t.tapID); status != 0 && firstErr == nil {
		firstErr = fmt.Errorf("destroy process tap: CoreAudio error %d", int(status))
	}
	return firstErr
}

