//go:build darwin

package platform

import "wakebit/internal/engine"

// New returns the macOS platform.
//
// TODO(Task 6): return darwinplat.New() from wakebit/internal/platform/darwin
// once that package exists; until then macOS gets the unsupported stub.
func New() engine.Platform { return unsupported{goos: "darwin"} }
