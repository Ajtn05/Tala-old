package audio

import "math"

// RMS computes the root mean square of a sample buffer.
func RMS(samples []float32) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sumSq float64
	for _, s := range samples {
		sumSq += float64(s) * float64(s)
	}
	return math.Sqrt(sumSq / float64(len(samples)))
}

// MixToMono averages all channels into one — whisper requires mono input.
func MixToMono(interleaved []float32, channels int) []float32 {
	if channels <= 1 {
		out := make([]float32, len(interleaved))
		copy(out, interleaved)
		return out
	}
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

// Resample resamples a mono buffer by the given ratio using linear interpolation.
func Resample(samples []float32, ratio float64) []float32 {
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
