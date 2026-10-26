//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
)

func startDetached(exe string) error {
	cmd := exec.Command(exe)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func reportError(msg string) { fmt.Fprintln(os.Stderr, "wakebit:", msg) }
