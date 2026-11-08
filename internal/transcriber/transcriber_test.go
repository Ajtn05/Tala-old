package transcriber

import (
	"context"
	"math"
	"testing"
)

func TestDigitalSilenceKeepsEveryNonzeroSample(t *testing.T) {
	for _, test := range []struct {
		name    string
		samples []float32
		want    bool
	}{
		{"empty", nil, false},
		{"zeros", make([]float32, 30*16000), true},
		{"negative zero", []float32{0, float32(math.Copysign(0, -1))}, true},
		{"quiet speech", []float32{0, 0.0000001, 0}, false},
		{"smallest positive sample", []float32{0, math.SmallestNonzeroFloat32}, false},
		{"smallest negative sample", []float32{-math.SmallestNonzeroFloat32, 0}, false},
		{"nonzero final sample", append(make([]float32, 30*16000-1), 0.01), false},
		{"nan", []float32{float32(math.NaN())}, false},
		{"infinity", []float32{float32(math.Inf(1))}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := digitalSilence(test.samples); got != test.want {
				t.Fatalf("digitalSilence = %v, want %v", got, test.want)
			}
		})
	}
}

func TestDigitalSilenceDoesNotInvokeNativeModel(t *testing.T) {
	trans := &Transcriber{} // Any attempt to use the model would panic.
	segments, err := trans.Transcribe(context.Background(), make([]float32, 16000))
	if err != nil || len(segments) != 0 {
		t.Fatalf("silence produced segments: %+v, %v", segments, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := trans.Transcribe(ctx, make([]float32, 16000)); err != context.Canceled {
		t.Fatalf("cancellation ignored for silence: %v", err)
	}
}
