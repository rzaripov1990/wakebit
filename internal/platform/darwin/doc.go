// Package darwinplat implements engine.Platform on macOS.
//
// UNVERIFIED: this package was written on Linux and has never been compiled
// or run on a Mac. Only the pure helpers (helpers.go) are unit-tested on
// Linux; the cgo glue (platform_darwin.go) is first compiled by the macOS CI
// job and must be checked by hand on a real Mac before it is trusted.
//
// Layout: helpers.go holds OS-independent logic (window-name matching,
// AppleScript snippets and output parsing, idle bookkeeping, UTF-16
// splitting) so it builds and tests on any GOOS; platform_darwin.go holds the
// cgo calls into IOKit / CoreGraphics / ApplicationServices and the
// open/osascript invocations.
package darwinplat
