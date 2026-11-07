package paths

import (
	"os"
	"path/filepath"
	"strings"

	"tala/internal/config"
)

// ResolveForBundle returns the model path and output directory, adjusted for
// .app bundle layout when running inside one. For dev builds (not inside a
// bundle), it returns the config defaults.
func ResolveForBundle() (modelPath string, outputDir string) {
	exe, err := os.Executable()
	if err != nil {
		return config.DefaultModelPath, config.DefaultOutputDir
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return config.DefaultModelPath, config.DefaultOutputDir
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}
	return resolveForExe(exe, home)
}

func resolveForExe(exe, home string) (string, string) {
	dir := filepath.Dir(exe)
	if !strings.HasSuffix(dir, filepath.Join("Contents", "MacOS")) {
		return config.DefaultModelPath, config.DefaultOutputDir
	}

	resourcesDir := filepath.Join(filepath.Dir(dir), "Resources")
	return filepath.Join(resourcesDir, config.DefaultModelPath), filepath.Join(home, "Documents", "Kian")
}

// ResolveDir resolves a directory path. If dir is empty, the config default
// is used. If dir is relative, it is resolved relative to the executable.
func ResolveDir(dir string) string {
	if dir == "" {
		dir = config.DefaultOutputDir
	}
	if filepath.IsAbs(dir) {
		return dir
	}
	exe, err := os.Executable()
	if err != nil {
		return dir
	}
	return filepath.Join(filepath.Dir(exe), dir)
}
