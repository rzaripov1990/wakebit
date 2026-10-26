package main

import (
	"fmt"
	"os"

	"wakebit/internal/install"
)

// runInstall copies the exe into the install dir, enables autostart and
// launches the installed copy.
func runInstall() error {
	dir, err := install.InstallDir()
	if err != nil {
		return err
	}
	exe, err := install.CopySelf(dir)
	if err != nil {
		return err
	}
	if err := install.SetAutostart(exe, true); err != nil {
		return fmt.Errorf("autostart: %w", err)
	}
	if err := startDetached(exe); err != nil {
		return fmt.Errorf("start %s: %w", exe, err)
	}
	return nil
}

// runUninstall disables autostart and removes the install dir (best effort).
func runUninstall() error {
	err := install.SetAutostart("", false)
	dir, derr := install.InstallDir()
	if derr != nil {
		return derr
	}
	self, _ := os.Executable()
	if rerr := install.RemoveDir(dir, self); rerr != nil && err == nil {
		err = rerr
	}
	return err
}
