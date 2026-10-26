// Package config persists Wakebit settings as JSON.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Mode string

const (
	ModeFull  Mode = "full"
	ModeLight Mode = "light"
)

type Config struct {
	Mode        Mode
	Working     bool
	IntervalSec int
}

const defaultIntervalSec = 60

// maxIntervalSec caps the interval at one day; larger values are clamped
// (they would also overflow time.Duration arithmetic when huge).
const maxIntervalSec = 86400

func Default() Config {
	return Config{Mode: ModeLight, Working: false, IntervalSec: defaultIntervalSec}
}

// Load never fails: missing, corrupt or invalid fields fall back to defaults.
func Load(path string) Config {
	d := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		return d
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return d
	}
	if c.Mode != ModeFull && c.Mode != ModeLight {
		c.Mode = d.Mode
	}
	if c.IntervalSec <= 0 {
		c.IntervalSec = d.IntervalSec
	}
	if c.IntervalSec > maxIntervalSec {
		c.IntervalSec = maxIntervalSec
	}
	return c
}

func Save(path string, c Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "Wakebit", "config.json")
}
