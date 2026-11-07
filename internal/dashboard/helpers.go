//go:build darwin

package dashboard

import (
	"context"
	"tala/internal/audio"
	"tala/internal/media"
	"time"
)

func audioMode(i int) audio.Mode { return audio.Mode(i) }
func contextTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 2*time.Minute)
}

var probe = media.Probe
var fetch = media.Fetch
