//go:build windows

package install

import (
	"errors"

	"golang.org/x/sys/windows/registry"
)

const (
	runKey   = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValue = "Wakebit"
)

// SetAutostart adds or removes the HKCU Run entry pointing at exePath.
func SetAutostart(exePath string, on bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !on {
		if err := k.DeleteValue(runValue); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return err
		}
		return nil
	}
	return k.SetStringValue(runValue, `"`+exePath+`"`)
}

// AutostartEnabled reports whether the Run entry exists.
func AutostartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(runValue)
	return err == nil
}
