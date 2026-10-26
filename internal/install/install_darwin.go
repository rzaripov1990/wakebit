//go:build darwin

package install

// UNVERIFIED: never run on a Mac yet.

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// InstallDir returns ~/Applications/Wakebit (always absolute, or an error).
// Its base name is "Wakebit", so RemoveDir's guard accepts it.
func InstallDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.New("install: cannot determine the home directory")
	}
	return homeInstallDir(home)
}

func agentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.New("install: cannot determine the home directory")
	}
	return launchAgentPath(home)
}

// SetAutostart writes (on) or removes (off) the user LaunchAgent
// ~/Library/LaunchAgents/kz.wakebit.plist that starts exePath at login. It
// does not call launchctl: the change takes effect at the next login.
func SetAutostart(exePath string, on bool) error {
	p, err := agentPath()
	if err != nil {
		return err
	}
	if !on {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	if !filepath.IsAbs(exePath) {
		return errors.New("install: autostart needs an absolute executable path")
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, renderLaunchAgent(exePath), 0o644)
}

// AutostartEnabled reports whether the LaunchAgent plist exists.
func AutostartEnabled() bool {
	p, err := agentPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}
