package config

import "time"

// App defaults.
const (
	DefaultModelPath = "models/ggml-large-v3-turbo.bin"
	DefaultOutputDir = "output"
	DefaultLanguage  = "auto"
	DefaultThreads   = 4
)

// Source labels used in transcript output to attribute speech
// to either the local microphone or remote system audio.
const (
	LabelMicrophone  = "Microphone"
	LabelSystemAudio = "System Audio"
	LabelUnknown     = "Unknown"
)

// Audio capture tuning.
const (
	// FramesPerBuffer is the number of audio frames read per PortAudio
	// callback. Lower values reduce latency but increase CPU overhead.
	DefaultFramesPerBuffer = 1024

	// SilenceThreshold is the RMS amplitude below which a buffer is
	// treated as silence. Raise it in noisy environments to avoid
	// capturing background hum; lower it for quiet speakers.
	DefaultSilenceThreshold = 0.003

	// SilenceDuration is how many seconds of consecutive silence trigger
	// a chunk flush (end of phrase). Shorter values produce more frequent,
	// smaller transcription segments; longer values merge more speech
	// into a single chunk.
	DefaultSilenceDuration = 1.5

	// MaxChunkDuration is the maximum number of seconds of audio in a
	// single chunk before it is force-flushed to the transcriber. This
	// prevents unbounded memory growth during long uninterrupted speech.
	DefaultMaxChunkDuration = 30.0

	// DominanceRatio controls speaker separation in aggregate (dual-source)
	// mode. When both channels have speech, the weaker channel is suppressed
	// only if the stronger channel's RMS energy exceeds it by this factor.
	// Higher values keep both speakers more often; lower values are more
	// aggressive at isolating the dominant speaker.
	DefaultDominanceRatio = 2.0

	// CrosstalkThreshold is the envelope correlation above which two
	// channels are considered to carry the same signal (crosstalk).
	// When detected, the system audio (remote) channel is emitted
	// because it carries the original digital signal; the mic copy
	// is suppressed. Range 0–1. Acoustic crosstalk (speaker → room
	// → mic) typically yields envelope correlations of 0.3–0.6 due
	// to reverb and frequency response distortion, so this must be
	// set lower than one might expect for digital signals.
	DefaultCrosstalkThreshold = 0.3
)

// SilenceFlush is the default interval after which a pending transcription
// segment is flushed to disk if no new speech arrives.
const DefaultSilenceFlush = 5 * time.Second
