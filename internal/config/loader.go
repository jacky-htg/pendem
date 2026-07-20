package config

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
)

// DefaultConfigPaths returns list of paths to search for config file
func DefaultConfigPaths() []string {
	paths := []string{}

	// 1. Current directory (always)
	paths = append(paths, "pendem.conf")
	paths = append(paths, "config/pendem.conf")

	// 2. User home
	if home := os.Getenv("HOME"); home != "" {
		paths = append(paths, filepath.Join(home, ".pendem", "pendem.conf"))
		paths = append(paths, filepath.Join(home, ".config", "pendem", "pendem.conf"))
	}

	// 3. System directories
	switch runtime.GOOS {
	case "windows":
		if programData := os.Getenv("ProgramData"); programData != "" {
			paths = append(paths, filepath.Join(programData, "Pendem", "pendem.conf"))
		}
		if appData := os.Getenv("APPDATA"); appData != "" {
			paths = append(paths, filepath.Join(appData, "Pendem", "pendem.conf"))
		}
	case "darwin":
		// Mac: /usr/local/etc
		paths = append(paths, "/usr/local/etc/pendem/pendem.conf")
		paths = append(paths, "/etc/pendem/pendem.conf")
	default:
		// Linux/Unix
		paths = append(paths, "/etc/pendem/pendem.conf")
		paths = append(paths, "/usr/local/etc/pendem/pendem.conf")
		paths = append(paths, "/opt/pendem/etc/pendem.conf")
	}

	return paths
}

// FindConfigFile searches for config file in default locations
func FindConfigFile(path string) (string, error) {
	// If user specified path, use it first
	if path != "" {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		// If specified path doesn't exist, continue searching
		log.Printf("⚠️ Specified config file not found: %s", path)
	}

	// Search default locations
	for _, p := range DefaultConfigPaths() {
		if _, err := os.Stat(p); err == nil {
			log.Printf("✅ Found config file: %s", p)
			return p, nil
		}
	}

	return "", os.ErrNotExist
}
