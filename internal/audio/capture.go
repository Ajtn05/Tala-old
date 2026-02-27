package audio

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/gordonklaus/portaudio"

	"kian/internal/config"
)

// TargetSampleRate is the sample rate required by whisper.cpp. This is not
// configurable — the model always expects 16 kHz input.
const TargetSampleRate = 16000

type Chunk struct {
	Source      string
	Samples     []float32
	StartOffset time.Duration
}

type CaptureConfig struct {
	Device      *portaudio.DeviceInfo
	SampleRate  float64
	SourceLabel string

	FramesPerBuffer    int
	SilenceThreshold   float64
	SilenceDuration    float64
	MaxChunkDuration   float64
	DominanceRatio     float64
	CrosstalkThreshold float64
}

func (c CaptureConfig) framesPerBuffer() int {
	if c.FramesPerBuffer > 0 {
		return c.FramesPerBuffer
	}
	return config.DefaultFramesPerBuffer
}

func (c CaptureConfig) silenceThreshold() float64 {
	if c.SilenceThreshold > 0 {
		return c.SilenceThreshold
	}
	return config.DefaultSilenceThreshold
}

func (c CaptureConfig) silenceDuration() float64 {
	if c.SilenceDuration > 0 {
		return c.SilenceDuration
	}
	return config.DefaultSilenceDuration
}

func (c CaptureConfig) maxChunkDuration() float64 {
	if c.MaxChunkDuration > 0 {
		return c.MaxChunkDuration
	}
	return config.DefaultMaxChunkDuration
}

func (c CaptureConfig) dominanceRatio() float64 {
	if c.DominanceRatio > 0 {
		return c.DominanceRatio
	}
	return config.DefaultDominanceRatio
}

func (c CaptureConfig) crosstalkThreshold() float64 {
	if c.CrosstalkThreshold > 0 {
		return c.CrosstalkThreshold
	}
	return config.DefaultCrosstalkThreshold
}

type capturer struct {
	out chan<- Chunk

	sampleRate  float64
	fpb         int
	numChannels int
	aggregate   bool

	needResample bool
	ratio        float64

	silThresh      float64
	silenceFrames  int
	maxFrames      int
	domRatio       float64
	crosstalkThresh float64
	sourceLabel    string

	accumulator   []float32
	silentCount   int
	totalFrames   int
	elapsedFrames int
	hasSpeech     bool
}

func Capture(ctx context.Context, cfg CaptureConfig, out chan<- Chunk) error {
	sampleRate := cfg.SampleRate
	if sampleRate == 0 {
		sampleRate = cfg.Device.DefaultSampleRate
	}

	fpb := cfg.framesPerBuffer()

	stream, buf, err := openStream(cfg.Device, sampleRate, fpb)
	if err != nil {
		return err
	}
	defer stream.Close()

	if err := stream.Start(); err != nil {
		return fmt.Errorf("start stream: %w", err)
	}
	defer stream.Stop()

	c := &capturer{
		out:           out,
		sampleRate:    sampleRate,
		fpb:           fpb,
		numChannels:   cfg.Device.MaxInputChannels,
		aggregate:     cfg.Device.MaxInputChannels > 2,
		needResample:  math.Abs(sampleRate-TargetSampleRate) > 1.0,
		ratio:         TargetSampleRate / sampleRate,
		silThresh:       cfg.silenceThreshold(),
		silenceFrames:   int(sampleRate * cfg.silenceDuration()),
		maxFrames:       int(sampleRate * cfg.maxChunkDuration()),
		domRatio:        cfg.dominanceRatio(),
		crosstalkThresh: cfg.crosstalkThreshold(),
		sourceLabel:     cfg.SourceLabel,
		accumulator:   make([]float32, 0, int(sampleRate*5)*cfg.Device.MaxInputChannels),
	}

	return c.readLoop(ctx, stream, buf)
}

func openStream(device *portaudio.DeviceInfo, sampleRate float64, fpb int) (*portaudio.Stream, []float32, error) {
	buf := make([]float32, fpb*device.MaxInputChannels)
	params := portaudio.StreamParameters{
		Input: portaudio.StreamDeviceParameters{
			Device:   device,
			Channels: device.MaxInputChannels,
			Latency:  device.DefaultHighInputLatency,
		},
		SampleRate:      sampleRate,
		FramesPerBuffer: fpb,
	}

	stream, err := portaudio.OpenStream(params, buf)
	if err != nil {
		return nil, nil, fmt.Errorf("open stream: %w", err)
	}
	return stream, buf, nil
}

func (c *capturer) readLoop(ctx context.Context, stream *portaudio.Stream, buf []float32) error {
	for {
		select {
		case <-ctx.Done():
			c.flushRemaining()
			return nil
		default:
		}

		if err := stream.Read(); err != nil {
			if strings.Contains(err.Error(), "Input overflowed") {
				log.Println("warning: input overflowed, skipping buffer")
				continue
			}
			return fmt.Errorf("read stream: %w", err)
		}

		c.processBuffer(buf)
	}
}

func (c *capturer) processBuffer(buf []float32) {
	hasSpeechInBuf := hasSourceSpeech(buf, c.numChannels, c.aggregate, c.silThresh)
	c.elapsedFrames += c.fpb

	if hasSpeechInBuf {
		c.silentCount = 0
		c.hasSpeech = true
	} else {
		c.silentCount += c.fpb
	}

	if c.hasSpeech {
		c.accumulator = append(c.accumulator, buf...)
		c.totalFrames += c.fpb
	}

	if !c.hasSpeech {
		return
	}

	if c.silentCount < c.silenceFrames && c.totalFrames < c.maxFrames {
		return
	}

	c.flushAccumulator()
}

func (c *capturer) startTime() time.Duration {
	return time.Duration(float64(c.elapsedFrames-c.totalFrames)/c.sampleRate*1000) * time.Millisecond
}

func (c *capturer) flushRemaining() {
	if !c.hasSpeech {
		return
	}
	c.emitChunks(c.accumulator, c.startTime())
}

func (c *capturer) flushAccumulator() {
	c.emitChunks(c.accumulator, c.startTime())
	c.accumulator = c.accumulator[:0]
	c.silentCount = 0
	c.totalFrames = 0
	c.hasSpeech = false
}

func (c *capturer) isSilent(samples []float32) bool {
	return rms(samples) < c.silThresh
}

func (c *capturer) emitChunks(interleaved []float32, startOffset time.Duration) {
	if !c.aggregate {
		c.emitMono(interleaved, startOffset)
		return
	}
	c.emitStereo(interleaved, startOffset)
}

func (c *capturer) emitMono(interleaved []float32, startOffset time.Duration) {
	mono := mixToMono(interleaved, c.numChannels)
	if c.needResample {
		mono = resample(mono, c.ratio)
	}
	if c.isSilent(mono) {
		return
	}
	label := c.sourceLabel
	if label == "" {
		label = config.LabelUnknown
	}
	c.out <- Chunk{Source: label, Samples: mono, StartOffset: startOffset}
}

// emitStereo splits an aggregate buffer into remote (system audio) and local
// (microphone) signals. When both channels have speech, envelope correlation
// detects crosstalk (system audio leaking through speakers to the mic). If
// crosstalk is found the remote channel is always emitted because it carries
// the original digital signal. Otherwise the weaker channel is suppressed
// when the stronger exceeds it by domRatio; if neither dominates, both are
// emitted as separate chunks so the transcriber can attribute them to
// different speakers.
func (c *capturer) emitStereo(interleaved []float32, startOffset time.Duration) {
	remote, local := splitBySource(interleaved, c.numChannels)

	if c.needResample {
		remote = resample(remote, c.ratio)
		local = resample(local, c.ratio)
	}

	remoteSilent := c.isSilent(remote)
	localSilent := c.isSilent(local)

	if remoteSilent && localSilent {
		return
	}

	if !remoteSilent && !localSilent {
		remoteRMS := rms(remote)
		localRMS := rms(local)

		if crossCorrelation(remote, local) >= c.crosstalkThresh {
			c.out <- Chunk{Source: config.LabelSystemAudio, Samples: remote, StartOffset: startOffset}
			return
		}

		if remoteRMS >= localRMS*c.domRatio {
			c.out <- Chunk{Source: config.LabelSystemAudio, Samples: remote, StartOffset: startOffset}
			return
		}
		if localRMS >= remoteRMS*c.domRatio {
			c.out <- Chunk{Source: config.LabelMicrophone, Samples: local, StartOffset: startOffset}
			return
		}
	}

	if !remoteSilent {
		c.out <- Chunk{Source: config.LabelSystemAudio, Samples: remote, StartOffset: startOffset}
	}
	if !localSilent {
		c.out <- Chunk{Source: config.LabelMicrophone, Samples: local, StartOffset: startOffset}
	}
}

// hasSourceSpeech checks whether the interleaved audio buffer contains speech.
// In aggregate mode it computes per-source RMS (remote = all channels except
// the last, local = last channel); in single-source mode it checks the whole buffer.
func hasSourceSpeech(buf []float32, channels int, aggregate bool, silThresh float64) bool {
	if !aggregate {
		return rms(buf) >= silThresh
	}
	frames := len(buf) / channels
	lastCh := channels - 1

	var remoteSumSq, localSumSq float64
	for i := range frames {
		base := i * channels
		var remoteSum float32
		for ch := range lastCh {
			remoteSum += buf[base+ch]
		}
		r := float64(remoteSum / float32(lastCh))
		remoteSumSq += r * r

		l := float64(buf[base+lastCh])
		localSumSq += l * l
	}
	remoteRMS := math.Sqrt(remoteSumSq / float64(frames))
	localRMS := math.Sqrt(localSumSq / float64(frames))
	return remoteRMS >= silThresh || localRMS >= silThresh
}

// splitBySource separates an interleaved multi-channel buffer into two mono
// signals: remote (average of all channels except the last) and local (last
// channel only). This assumes an aggregate device where BlackHole feeds the
// first N-1 channels and the built-in microphone feeds the last channel.
func splitBySource(interleaved []float32, channels int) (remote []float32, local []float32) {
	frames := len(interleaved) / channels
	remote = make([]float32, frames)
	local = make([]float32, frames)

	lastCh := channels - 1
	for i := range frames {
		base := i * channels
		var sum float32
		for ch := range lastCh {
			sum += interleaved[base+ch]
		}
		remote[i] = sum / float32(lastCh)
		local[i] = interleaved[base+lastCh]
	}
	return remote, local
}

// envelopeWindowSamples is the number of samples per energy envelope frame.
// At 16 kHz this gives ~10 ms windows, producing a compact envelope sequence
// that is robust to room acoustics, reverb, and frequency response changes.
const envelopeWindowSamples = 160

// crossCorrelation computes the Pearson correlation between the energy
// envelopes of two signals. Each signal is divided into short frames and
// the RMS of each frame is computed, yielding a low-rate envelope. Correlating
// envelopes instead of raw waveforms is robust to the acoustic distortion
// that occurs when system audio travels from speakers through a room to the
// microphone (reverb, frequency response, multipath).
func crossCorrelation(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}

	envA := envelope(a)
	envB := envelope(b)
	return pearson(envA, envB)
}

// envelope computes the RMS energy in consecutive non-overlapping windows,
// producing a compact representation of the signal's amplitude over time.
func envelope(samples []float32) []float64 {
	n := len(samples) / envelopeWindowSamples
	if n == 0 {
		return nil
	}
	env := make([]float64, n)
	for i := range n {
		start := i * envelopeWindowSamples
		end := start + envelopeWindowSamples
		env[i] = rms(samples[start:end])
	}
	return env
}

// pearson computes the Pearson correlation coefficient between two sequences.
func pearson(a, b []float64) float64 {
	n := len(a)
	if n == 0 || len(b) != n {
		return 0
	}

	var sumA, sumB float64
	for i := range n {
		sumA += a[i]
		sumB += b[i]
	}
	meanA := sumA / float64(n)
	meanB := sumB / float64(n)

	var cov, varA, varB float64
	for i := range n {
		da := a[i] - meanA
		db := b[i] - meanB
		cov += da * db
		varA += da * da
		varB += db * db
	}

	denom := math.Sqrt(varA * varB)
	if denom < 1e-12 {
		return 0
	}
	return cov / denom
}

// rms computes the root mean square of the sample buffer — a measure of
// signal energy used to distinguish speech from silence.
func rms(samples []float32) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sumSq float64
	for _, s := range samples {
		sumSq += float64(s) * float64(s)
	}
	return math.Sqrt(sumSq / float64(len(samples)))
}

// mixToMono averages all channels of an interleaved buffer into a single
// mono signal. Used for single-source capture where we don't need to
// separate speakers.
func mixToMono(interleaved []float32, channels int) []float32 {
	frames := len(interleaved) / channels
	mono := make([]float32, frames)
	for i := range frames {
		var sum float32
		for ch := range channels {
			sum += interleaved[i*channels+ch]
		}
		mono[i] = sum / float32(channels)
	}
	return mono
}

// resample converts audio from one sample rate to another using linear
// interpolation. The ratio is targetRate/sourceRate (e.g. 16000/44100).
// This is needed because whisper.cpp expects 16 kHz input but most audio
// devices capture at 44.1 or 48 kHz.
func resample(samples []float32, ratio float64) []float32 {
	outLen := int(float64(len(samples)) * ratio)
	out := make([]float32, outLen)
	for i := range out {
		srcIdx := float64(i) / ratio
		idx := int(srcIdx)
		frac := float32(srcIdx - float64(idx))
		if idx+1 < len(samples) {
			out[i] = samples[idx]*(1-frac) + samples[idx+1]*frac
		} else if idx < len(samples) {
			out[i] = samples[idx]
		}
	}
	return out
}
