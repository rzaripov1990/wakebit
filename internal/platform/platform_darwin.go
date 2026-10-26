//go:build darwin

package platform

import (
	"wakebit/internal/engine"
	darwinplat "wakebit/internal/platform/darwin"
)

// New returns the macOS platform (UNVERIFIED: see internal/platform/darwin).
func New() engine.Platform { return darwinplat.New() }
