//go:build !windows && !darwin

package install

// InstallDir returns $LOCALAPPDATA/Wakebit, falling back to
// ~/AppData/Local/Wakebit (always absolute, or an error).
func InstallDir() (string, error) { return localAppDataInstallDir() }

// SetAutostart is unsupported on this OS.
func SetAutostart(exePath string, on bool) error { return ErrUnsupported }

// AutostartEnabled always reports false on this OS.
func AutostartEnabled() bool { return false }
