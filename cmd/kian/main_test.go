package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"kian/internal/config"
)

func TestResolvePaths(t *testing.T) {
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
			modelPath, outputDir := resolvePaths(tt.exe, tt.home)
			assert.Equal(t, tt.wantModelPath, modelPath)
			assert.Equal(t, tt.wantOutputDir, outputDir)
		})
	}
}
