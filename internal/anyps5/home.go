// SPDX-License-Identifier: GPL-2.0-only

package anyps5

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Home is the directory AnyGame keeps its installed runtimes, cache and state
// in. ANYGAME_HOME overrides the platform default.
func Home() (string, error) {
	if value := os.Getenv("ANYGAME_HOME"); value != "" {
		return filepath.Abs(value)
	}
	switch runtime.GOOS {
	case "windows":
		if value := os.Getenv("LOCALAPPDATA"); value != "" {
			return filepath.Join(value, "AnyGame"), nil
		}
		base, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(base, "AnyGame"), nil
	default:
		if value := os.Getenv("XDG_DATA_HOME"); value != "" {
			return filepath.Join(value, "AnyGame"), nil
		}
		base, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(base, ".local", "share", "AnyGame"), nil
	}
}

// VersionsDir is where one directory per AnyPS5 release lives.
func VersionsDir() (string, error) {
	home, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "runtimes"), nil
}

// CacheDir holds downloaded release assets.
func CacheDir() (string, error) {
	home, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "cache"), nil
}

// NormalizeTarget maps user input and host names to the target names used by
// AnyPS5 release assets.
func NormalizeTarget(value string) (string, error) {
	switch value {
	case "", "host":
		return runtime.GOOS, nil
	case "linux", "windows":
		return value, nil
	default:
		return "", fmt.Errorf("unsupported target %q: use linux or windows", value)
	}
}

// HostTarget is the platform AnyGame itself runs on.
func HostTarget() string {
	return runtime.GOOS
}
