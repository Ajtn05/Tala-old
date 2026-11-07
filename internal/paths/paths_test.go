package paths

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"tala/internal/config"
)

func TestResolveForExe(t *testing.T) {
	tests := []struct {
		name          string
		exe           string
		home          string
		wantModelPath string
		wantOutputDir string
	}{
		{
			name:          "inside app bundle",
			exe:           "/Applications/Kian.app/Contents/MacOS/kian",
			home:          "/Users/alice",
			wantModelPath: filepath.Join("/Applications/Kian.app/Contents/Resources", config.DefaultModelPath),
			wantOutputDir: "/Users/alice/Documents/Kian",
		},
		{
			name:          "outside app bundle",
			exe:           "/usr/local/bin/kian",
			home:          "/Users/alice",
			wantModelPath: config.DefaultModelPath,
			wantOutputDir: config.DefaultOutputDir,
		},
		{
			name:          "dev build in project dir",
			exe:           "/Users/alice/projects/kian/kian",
			home:          "/Users/alice",
			wantModelPath: config.DefaultModelPath,
			wantOutputDir: config.DefaultOutputDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelPath, outputDir := resolveForExe(tt.exe, tt.home)
			assert.Equal(t, tt.wantModelPath, modelPath)
			assert.Equal(t, tt.wantOutputDir, outputDir)
		})
	}
}

func TestResolveDir(t *testing.T) {
	tests := []struct {
		name string
		dir  string
		abs  bool
	}{
		{
			name: "absolute path unchanged",
			dir:  "/tmp/output",
			abs:  true,
		},
		{
			name: "empty uses default",
			dir:  "",
			abs:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveDir(tt.dir)
			if tt.abs {
				assert.Equal(t, tt.dir, got)
			} else {
				assert.NotEmpty(t, got)
			}
		})
	}
}
