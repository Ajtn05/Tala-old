package audio

/*
#cgo LDFLAGS: -framework CoreAudio -framework CoreFoundation

#include <CoreAudio/CoreAudio.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>
#include <string.h>

// deviceInfo holds the name and UID of an audio device.
typedef struct {
	char name[256];
	char uid[256];
	int  maxInputChannels;
	int  maxOutputChannels;
} deviceInfo;

// listDevices enumerates all CoreAudio devices into the provided buffer.
// Returns the number of devices found, or -1 on error.
static int listDevices(deviceInfo *buf, int bufLen) {
	AudioObjectPropertyAddress addr = {
		kAudioHardwarePropertyDevices,
		kAudioObjectPropertyScopeGlobal,
		kAudioObjectPropertyElementMain,
	};

	UInt32 dataSize = 0;
	OSStatus status = AudioObjectGetPropertyDataSize(
		kAudioObjectSystemObject, &addr, 0, NULL, &dataSize);
	if (status != noErr) return -1;

	int count = (int)(dataSize / sizeof(AudioDeviceID));
	if (count > bufLen) count = bufLen;

	AudioDeviceID *devices = (AudioDeviceID *)malloc(dataSize);
	status = AudioObjectGetPropertyData(
		kAudioObjectSystemObject, &addr, 0, NULL, &dataSize, devices);
	if (status != noErr) {
		free(devices);
		return -1;
	}

	for (int i = 0; i < count; i++) {
		memset(&buf[i], 0, sizeof(deviceInfo));

		// Get device name.
		CFStringRef cfName = NULL;
		UInt32 size = sizeof(cfName);
		AudioObjectPropertyAddress nameAddr = {
			kAudioObjectPropertyName,
			kAudioObjectPropertyScopeGlobal,
			kAudioObjectPropertyElementMain,
		};
		if (AudioObjectGetPropertyData(devices[i], &nameAddr, 0, NULL, &size, &cfName) == noErr && cfName) {
			CFStringGetCString(cfName, buf[i].name, sizeof(buf[i].name), kCFStringEncodingUTF8);
			CFRelease(cfName);
		}

		// Get device UID.
		CFStringRef cfUID = NULL;
		size = sizeof(cfUID);
		AudioObjectPropertyAddress uidAddr = {
			kAudioDevicePropertyDeviceUID,
			kAudioObjectPropertyScopeGlobal,
			kAudioObjectPropertyElementMain,
		};
		if (AudioObjectGetPropertyData(devices[i], &uidAddr, 0, NULL, &size, &cfUID) == noErr && cfUID) {
			CFStringGetCString(cfUID, buf[i].uid, sizeof(buf[i].uid), kCFStringEncodingUTF8);
			CFRelease(cfUID);
		}

		// Get input channel count.
		AudioObjectPropertyAddress inputAddr = {
			kAudioDevicePropertyStreamConfiguration,
			kAudioDevicePropertyScopeInput,
			kAudioObjectPropertyElementMain,
		};
		UInt32 propSize = 0;
		if (AudioObjectGetPropertyDataSize(devices[i], &inputAddr, 0, NULL, &propSize) == noErr && propSize > 0) {
			AudioBufferList *abl = (AudioBufferList *)malloc(propSize);
			if (AudioObjectGetPropertyData(devices[i], &inputAddr, 0, NULL, &propSize, abl) == noErr) {
				for (UInt32 b = 0; b < abl->mNumberBuffers; b++) {
					buf[i].maxInputChannels += (int)abl->mBuffers[b].mNumberChannels;
				}
			}
			free(abl);
		}

		// Get output channel count.
		AudioObjectPropertyAddress outputAddr = {
			kAudioDevicePropertyStreamConfiguration,
			kAudioDevicePropertyScopeOutput,
			kAudioObjectPropertyElementMain,
		};
		propSize = 0;
		if (AudioObjectGetPropertyDataSize(devices[i], &outputAddr, 0, NULL, &propSize) == noErr && propSize > 0) {
			AudioBufferList *abl = (AudioBufferList *)malloc(propSize);
			if (AudioObjectGetPropertyData(devices[i], &outputAddr, 0, NULL, &propSize, abl) == noErr) {
				for (UInt32 b = 0; b < abl->mNumberBuffers; b++) {
					buf[i].maxOutputChannels += (int)abl->mBuffers[b].mNumberChannels;
				}
			}
			free(abl);
		}
	}

	free(devices);
	return count;
}

// createAggregateOrMultiOutput creates an aggregate or multi-output device.
// isStacked=0 for aggregate (combined input channels), isStacked=1 for multi-output.
// masterUID is the clock source device, secondUID gets drift compensation.
static OSStatus createAggregateOrMultiOutput(
	const char *name, const char *deviceUID,
	const char *masterUID, const char *secondUID,
	int isStacked, AudioDeviceID *outID)
{
	CFStringRef cfName      = CFStringCreateWithCString(kCFAllocatorDefault, name, kCFStringEncodingUTF8);
	CFStringRef cfUID       = CFStringCreateWithCString(kCFAllocatorDefault, deviceUID, kCFStringEncodingUTF8);
	CFStringRef cfMasterUID = CFStringCreateWithCString(kCFAllocatorDefault, masterUID, kCFStringEncodingUTF8);
	CFStringRef cfSecondUID = CFStringCreateWithCString(kCFAllocatorDefault, secondUID, kCFStringEncodingUTF8);

	// Master sub-device (no drift compensation — it's the clock source).
	const void *masterKeys[] = { CFSTR("uid") };
	const void *masterVals[] = { cfMasterUID };
	CFDictionaryRef masterSub = CFDictionaryCreate(
		kCFAllocatorDefault, masterKeys, masterVals, 1,
		&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);

	// Second sub-device (with drift compensation).
	int driftVal = 1;
	CFNumberRef cfDrift = CFNumberCreate(kCFAllocatorDefault, kCFNumberIntType, &driftVal);
	const void *secondKeys[] = { CFSTR("uid"), CFSTR("drift") };
	const void *secondVals[] = { cfSecondUID, cfDrift };
	CFDictionaryRef secondSub = CFDictionaryCreate(
		kCFAllocatorDefault, secondKeys, secondVals, 2,
		&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);

	const void *subs[] = { masterSub, secondSub };
	CFArrayRef subList = CFArrayCreate(kCFAllocatorDefault, subs, 2, &kCFTypeArrayCallBacks);

	CFNumberRef cfStacked = CFNumberCreate(kCFAllocatorDefault, kCFNumberIntType, &isStacked);
	int privateVal = 0;
	CFNumberRef cfPrivate = CFNumberCreate(kCFAllocatorDefault, kCFNumberIntType, &privateVal);

	const void *descKeys[] = {
		CFSTR("name"),
		CFSTR("uid"),
		CFSTR("subdevices"),
		CFSTR("master"),
		CFSTR("stacked"),
		CFSTR("private"),
	};
	const void *descVals[] = {
		cfName,
		cfUID,
		subList,
		cfMasterUID,
		cfStacked,
		cfPrivate,
	};
	CFDictionaryRef desc = CFDictionaryCreate(
		kCFAllocatorDefault, descKeys, descVals, 6,
		&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);

	OSStatus status = AudioHardwareCreateAggregateDevice(desc, outID);

	CFRelease(desc);
	CFRelease(cfPrivate);
	CFRelease(cfStacked);
	CFRelease(subList);
	CFRelease(secondSub);
	CFRelease(masterSub);
	CFRelease(cfDrift);
	CFRelease(cfSecondUID);
	CFRelease(cfMasterUID);
	CFRelease(cfUID);
	CFRelease(cfName);

	return status;
}
*/
import "C"

import (
	"fmt"
	"strings"
	"unsafe"
)

const (
	kianAggregateDeviceName = "Kian Aggregate"
	kianAggregateDeviceUID  = "com.kian.aggregate"
	kianMultiOutputName     = "Kian Multi-Output"
	kianMultiOutputUID      = "com.kian.multioutput"

	maxDevices = 128
)

type coreAudioDevice struct {
	Name             string
	UID              string
	MaxInputChannels int
	MaxOutputChannels int
}

func listCoreAudioDevices() ([]coreAudioDevice, error) {
	var buf [maxDevices]C.deviceInfo
	n := C.listDevices(&buf[0], C.int(maxDevices))
	if n < 0 {
		return nil, fmt.Errorf("enumerate CoreAudio devices")
	}

	devices := make([]coreAudioDevice, int(n))
	for i := 0; i < int(n); i++ {
		devices[i] = coreAudioDevice{
			Name:             C.GoString(&buf[i].name[0]),
			UID:              C.GoString(&buf[i].uid[0]),
			MaxInputChannels: int(buf[i].maxInputChannels),
			MaxOutputChannels: int(buf[i].maxOutputChannels),
		}
	}
	return devices, nil
}

// findDeviceUID returns the UID of the first device whose name contains
// the given substring (case-insensitive).
func findDeviceUID(nameSubstring string) (string, error) {
	devices, err := listCoreAudioDevices()
	if err != nil {
		return "", err
	}

	needle := strings.ToLower(nameSubstring)
	for _, d := range devices {
		if strings.Contains(strings.ToLower(d.Name), needle) {
			return d.UID, nil
		}
	}
	return "", fmt.Errorf("device %q not found", nameSubstring)
}

// DeviceStatus reports which audio devices are available.
type DeviceStatus struct {
	BlackHoleFound  bool
	AggregateFound  bool
	MultiOutputFound bool
	MicrophoneFound bool
}

// CheckDevices enumerates CoreAudio devices and reports which ones needed
// by Kian are present.
func CheckDevices() DeviceStatus {
	devices, err := listCoreAudioDevices()
	if err != nil {
		return DeviceStatus{}
	}

	var status DeviceStatus
	for _, d := range devices {
		name := strings.ToLower(d.Name)
		if strings.Contains(name, "blackhole") {
			status.BlackHoleFound = true
		}
		if strings.Contains(name, "aggregate") {
			status.AggregateFound = true
		}
		if strings.Contains(name, "multi-output") {
			status.MultiOutputFound = true
		}
		if strings.Contains(name, "microphone") {
			status.MicrophoneFound = true
		}
	}
	return status
}

// CreateKianAggregateDevice creates an aggregate input device combining
// BlackHole 2ch and the built-in microphone. This is used for ModeBoth
// to capture both system audio and microphone input simultaneously.
func CreateKianAggregateDevice() error {
	blackholeUID, err := findDeviceUID("blackhole")
	if err != nil {
		return fmt.Errorf("find BlackHole device: %w", err)
	}

	micUID, err := findDeviceUID("microphone")
	if err != nil {
		return fmt.Errorf("find microphone device: %w", err)
	}

	cName := C.CString(kianAggregateDeviceName)
	defer C.free(unsafe.Pointer(cName))
	cUID := C.CString(kianAggregateDeviceUID)
	defer C.free(unsafe.Pointer(cUID))
	cMaster := C.CString(blackholeUID)
	defer C.free(unsafe.Pointer(cMaster))
	cSecond := C.CString(micUID)
	defer C.free(unsafe.Pointer(cSecond))

	var deviceID C.AudioDeviceID
	status := C.createAggregateOrMultiOutput(cName, cUID, cMaster, cSecond, 0, &deviceID)
	if status != 0 {
		return fmt.Errorf("CoreAudio error %d", int(status))
	}
	return nil
}

// CreateKianMultiOutputDevice creates a multi-output device that routes
// system audio to both the built-in speakers and BlackHole 2ch. The user
// must set this as their system output for system audio capture to work.
func CreateKianMultiOutputDevice() error {
	// Find built-in output (speakers/headphones).
	devices, err := listCoreAudioDevices()
	if err != nil {
		return fmt.Errorf("enumerate devices: %w", err)
	}

	var speakerUID string
	for _, d := range devices {
		name := strings.ToLower(d.Name)
		if d.MaxOutputChannels > 0 && (strings.Contains(name, "speaker") || strings.Contains(name, "headphone") || strings.Contains(name, "built-in")) {
			speakerUID = d.UID
			break
		}
	}
	if speakerUID == "" {
		return fmt.Errorf("built-in output device not found")
	}

	blackholeUID, err := findDeviceUID("blackhole")
	if err != nil {
		return fmt.Errorf("find BlackHole device: %w", err)
	}

	cName := C.CString(kianMultiOutputName)
	defer C.free(unsafe.Pointer(cName))
	cUID := C.CString(kianMultiOutputUID)
	defer C.free(unsafe.Pointer(cUID))
	cMaster := C.CString(speakerUID)
	defer C.free(unsafe.Pointer(cMaster))
	cSecond := C.CString(blackholeUID)
	defer C.free(unsafe.Pointer(cSecond))

	var deviceID C.AudioDeviceID
	status := C.createAggregateOrMultiOutput(cName, cUID, cMaster, cSecond, 1, &deviceID)
	if status != 0 {
		return fmt.Errorf("CoreAudio error %d", int(status))
	}
	return nil
}
