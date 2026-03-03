#import <CoreAudio/CoreAudio.h>
#import <CoreAudio/AudioHardwareTapping.h>
#import <CoreAudio/CATapDescription.h>
#import <CoreFoundation/CoreFoundation.h>
#import <Foundation/Foundation.h>
#include "tap_darwin.h"
#include <string.h>
#include <stdatomic.h>

OSStatus createGlobalTap(AudioObjectID *outTapID) {
	@autoreleasepool {
		CATapDescription *desc =
			[[CATapDescription alloc] initStereoGlobalTapButExcludeProcesses:@[]];
		if (!desc) return -1;

		OSStatus status = AudioHardwareCreateProcessTap(desc, outTapID);
		[desc release];
		return status;
	}
}

OSStatus getTapUUID(AudioObjectID tapID, char *buf, int bufLen) {
	AudioObjectPropertyAddress addr = {
		'tuid',
		kAudioObjectPropertyScopeGlobal,
		kAudioObjectPropertyElementMain,
	};

	CFStringRef uuid = NULL;
	UInt32 size = sizeof(uuid);
	OSStatus status = AudioObjectGetPropertyData(tapID, &addr, 0, NULL, &size, &uuid);
	if (status != noErr) return status;
	if (!uuid) return -1;

	CFStringGetCString(uuid, buf, bufLen, kCFStringEncodingUTF8);
	CFRelease(uuid);
	return noErr;
}

OSStatus createTapAggregate(
	const char *tapUUID,
	const char *deviceName,
	const char *deviceUID,
	AudioDeviceID *outID)
{
	destroyAggregateByUID(deviceUID);

	CFStringRef cfTapUUID = CFStringCreateWithCString(kCFAllocatorDefault, tapUUID, kCFStringEncodingUTF8);
	CFStringRef cfName    = CFStringCreateWithCString(kCFAllocatorDefault, deviceName, kCFStringEncodingUTF8);
	CFStringRef cfUID     = CFStringCreateWithCString(kCFAllocatorDefault, deviceUID, kCFStringEncodingUTF8);

	const void *tapKeys[] = { CFSTR("uid") };
	const void *tapVals[] = { cfTapUUID };
	CFDictionaryRef tapEntry = CFDictionaryCreate(
		kCFAllocatorDefault, tapKeys, tapVals, 1,
		&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);

	const void *tapListItems[] = { tapEntry };
	CFArrayRef tapList = CFArrayCreate(
		kCFAllocatorDefault, tapListItems, 1, &kCFTypeArrayCallBacks);

	int privateVal = 1;
	CFNumberRef cfPrivate = CFNumberCreate(kCFAllocatorDefault, kCFNumberIntType, &privateVal);

	CFMutableDictionaryRef desc = CFDictionaryCreateMutable(
		kCFAllocatorDefault, 0,
		&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);

	CFDictionarySetValue(desc, CFSTR("name"), cfName);
	CFDictionarySetValue(desc, CFSTR("uid"), cfUID);
	CFDictionarySetValue(desc, CFSTR("taps"), tapList);
	CFDictionarySetValue(desc, CFSTR("private"), cfPrivate);

	OSStatus status = AudioHardwareCreateAggregateDevice(desc, outID);

	CFRelease(desc);
	CFRelease(cfPrivate);
	CFRelease(tapList);
	CFRelease(tapEntry);
	CFRelease(cfUID);
	CFRelease(cfName);
	CFRelease(cfTapUUID);

	return status;
}

#define TAP_RING_SIZE (48000 * 2 * 10) // 10 sec at 48kHz stereo
static float tapRing[TAP_RING_SIZE];
static _Atomic uint64_t tapWritePos = 0;
static _Atomic uint64_t tapReadPos = 0;
static _Atomic int tapChannels = 0;
static AudioDeviceIOProcID tapIOProcID = NULL;

OSStatus startTapIO(AudioDeviceID aggID) {
	// Query channel count.
	AudioObjectPropertyAddress addr = {
		kAudioDevicePropertyStreamConfiguration,
		kAudioDevicePropertyScopeInput,
		kAudioObjectPropertyElementMain,
	};
	UInt32 propSize = 0;
	if (AudioObjectGetPropertyDataSize(aggID, &addr, 0, NULL, &propSize) != noErr) return -1;
	AudioBufferList *abl = (AudioBufferList *)malloc(propSize);
	int ch = 0;
	if (AudioObjectGetPropertyData(aggID, &addr, 0, NULL, &propSize, abl) == noErr) {
		for (UInt32 b = 0; b < abl->mNumberBuffers; b++)
			ch += (int)abl->mBuffers[b].mNumberChannels;
	}
	free(abl);
	atomic_store_explicit(&tapChannels, ch, memory_order_release);

	// Reset ring buffer.
	atomic_store_explicit(&tapWritePos, 0, memory_order_relaxed);
	atomic_store_explicit(&tapReadPos, 0, memory_order_relaxed);

	OSStatus status = AudioDeviceCreateIOProcIDWithBlock(
		&tapIOProcID, aggID, NULL,
		^(const AudioTimeStamp *inNow,
		  const AudioBufferList *inInputData,
		  const AudioTimeStamp *inInputTime,
		  AudioBufferList *outOutputData,
		  const AudioTimeStamp *inOutputTime) {
			if (!inInputData) return;
			uint64_t wp = atomic_load_explicit(&tapWritePos, memory_order_relaxed);
			uint64_t rp = atomic_load_explicit(&tapReadPos, memory_order_acquire);
			for (UInt32 b = 0; b < inInputData->mNumberBuffers; b++) {
				float *samples = (float *)inInputData->mBuffers[b].mData;
				int numSamples = inInputData->mBuffers[b].mDataByteSize / sizeof(float);
				for (int i = 0; i < numSamples; i++) {
					if (wp - rp >= TAP_RING_SIZE) {
						rp = wp - TAP_RING_SIZE + 1;
						atomic_store_explicit(&tapReadPos, rp, memory_order_relaxed);
					}
					tapRing[wp % TAP_RING_SIZE] = samples[i];
					wp++;
				}
			}
			atomic_store_explicit(&tapWritePos, wp, memory_order_release);
		});
	if (status != noErr) return status;

	return AudioDeviceStart(aggID, tapIOProcID);
}

int readTapSamples(float *buf, int maxSamples) {
	uint64_t wp = atomic_load_explicit(&tapWritePos, memory_order_acquire);
	uint64_t rp = atomic_load_explicit(&tapReadPos, memory_order_relaxed);
	int available = (int)(wp - rp);
	if (available <= 0) return 0;
	if (available > maxSamples) available = maxSamples;

	for (int i = 0; i < available; i++) {
		buf[i] = tapRing[rp % TAP_RING_SIZE];
		rp++;
	}
	atomic_store_explicit(&tapReadPos, rp, memory_order_release);
	return available;
}

int getTapChannels(void) {
	return atomic_load_explicit(&tapChannels, memory_order_acquire);
}

OSStatus stopTapIO(AudioDeviceID aggID) {
	if (!tapIOProcID) return noErr;
	AudioDeviceStop(aggID, tapIOProcID);
	OSStatus status = AudioDeviceDestroyIOProcID(aggID, tapIOProcID);
	tapIOProcID = NULL;
	return status;
}

OSStatus destroyTap(AudioObjectID tapID) {
	return AudioHardwareDestroyProcessTap(tapID);
}

OSStatus destroyAggregate(AudioDeviceID deviceID) {
	return AudioHardwareDestroyAggregateDevice(deviceID);
}

OSStatus destroyAggregateByUID(const char *deviceUID) {
	AudioObjectPropertyAddress addr = {
		kAudioHardwarePropertyDevices,
		kAudioObjectPropertyScopeGlobal,
		kAudioObjectPropertyElementMain,
	};
	UInt32 dataSize = 0;
	if (AudioObjectGetPropertyDataSize(kAudioObjectSystemObject, &addr, 0, NULL, &dataSize) != noErr)
		return noErr;

	int count = (int)(dataSize / sizeof(AudioDeviceID));
	AudioDeviceID *devices = (AudioDeviceID *)malloc(dataSize);
	if (AudioObjectGetPropertyData(kAudioObjectSystemObject, &addr, 0, NULL, &dataSize, devices) != noErr) {
		free(devices);
		return noErr;
	}

	for (int i = 0; i < count; i++) {
		CFStringRef cfUID = NULL;
		UInt32 uidSize = sizeof(cfUID);
		AudioObjectPropertyAddress uidAddr = {
			kAudioDevicePropertyDeviceUID,
			kAudioObjectPropertyScopeGlobal,
			kAudioObjectPropertyElementMain,
		};
		if (AudioObjectGetPropertyData(devices[i], &uidAddr, 0, NULL, &uidSize, &cfUID) == noErr && cfUID) {
			char buf[256];
			CFStringGetCString(cfUID, buf, sizeof(buf), kCFStringEncodingUTF8);
			CFRelease(cfUID);
			if (strcmp(buf, deviceUID) == 0) {
				AudioHardwareDestroyAggregateDevice(devices[i]);
				break;
			}
		}
	}

	free(devices);
	return noErr;
}
