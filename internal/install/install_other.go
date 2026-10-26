//go:build !windows

package install

// SetAutostart is unsupported on this OS (macOS arrives in a later task).
func SetAutostart(exePath string, on bool) error { return ErrUnsupported }

// AutostartEnabled always reports false on this OS.
func AutostartEnabled() bool { return false }
