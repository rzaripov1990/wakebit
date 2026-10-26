//go:build !windows && !darwin

package platform

import (
	"runtime"

	"wakebit/internal/engine"
)

// New returns a stub whose operations fail with ErrUnsupported.
func New() engine.Platform { return unsupported{goos: runtime.GOOS} }
