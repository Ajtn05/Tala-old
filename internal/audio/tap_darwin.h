#ifndef TAP_DARWIN_H
#define TAP_DARWIN_H

#include <CoreAudio/CoreAudio.h>

// createGlobalTap creates a process tap that captures all system audio.
OSStatus createGlobalTap(AudioObjectID *outTapID);

// getTapUUID reads the UUID string from a process tap.
OSStatus getTapUUID(AudioObjectID tapID, char *buf, int bufLen);

// createTapAggregate creates a private aggregate device that includes the given
// process tap. The aggregate has 2 stereo input channels from the tap.
OSStatus createTapAggregate(
	const char *tapUUID,
	const char *deviceName,
	const char *deviceUID,
	AudioDeviceID *outID);

// startTapIO sets up a CoreAudio IO proc to read from the aggregate device
// and write samples to an internal ring buffer. Call readTapSamples to consume.
OSStatus startTapIO(AudioDeviceID aggID);

// readTapSamples copies up to maxSamples from the ring buffer into buf.
// Returns the number of samples actually copied.
int readTapSamples(float *buf, int maxSamples);

// getTapChannels returns the number of input channels on the tap aggregate.
int getTapChannels(void);

// stopTapIO stops the IO proc.
OSStatus stopTapIO(AudioDeviceID aggID);

// destroyTap destroys a process tap.
OSStatus destroyTap(AudioObjectID tapID);

// destroyAggregate destroys an aggregate device.
OSStatus destroyAggregate(AudioDeviceID deviceID);

// destroyAggregateByUID finds and destroys an aggregate device by its UID.
// Returns noErr if destroyed or if no device with that UID exists.
OSStatus destroyAggregateByUID(const char *deviceUID);

#endif
