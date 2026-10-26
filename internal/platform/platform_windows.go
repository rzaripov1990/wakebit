//go:build windows

package platform

import (
	"wakebit/internal/engine"
	winplat "wakebit/internal/platform/windows"
)

// New returns the Windows platform.
func New() engine.Platform { return winplat.New() }
