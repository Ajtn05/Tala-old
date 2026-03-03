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

)

// SilenceFlush is the default interval after which a pending transcription
// segment is flushed to disk if no new speech arrives.
const DefaultSilenceFlush = 5 * time.Second
