package winplat

import (
	"errors"
	"log"

	"golang.org/x/sys/windows"
)

var (
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procSetProcessDPIAware            = user32.NewProc("SetProcessDPIAware")
)

// dpiAwarenessContextPerMonitorAwareV2 is DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2,
// the pseudo-handle ((DPI_AWARENESS_CONTEXT)-4).
const dpiAwarenessContextPerMonitorAwareV2 = ^uintptr(3)

// EnsureDPIAware makes the process DPI-aware (per-monitor v2 where
// available, system-aware otherwise) so GetCursorPos, SetCursorPos and
// GetSystemMetrics work in physical pixels on mixed-DPI desktops. It must run
// before any window is created; it is idempotent and safe to call from any
// goroutine.
func EnsureDPIAware() { ensureDPIAware() }

var ensureDPIAware = ensureOnce(setDPIAware)

func setDPIAware() {
	// SetProcessDpiAwarenessContext: Windows 10 1607+; PER_MONITOR_AWARE_V2
	// needs 1703+ (older builds reject it with ERROR_INVALID_PARAMETER).
	if procSetProcessDpiAwarenessContext.Find() == nil {
		r, _, e := procSetProcessDpiAwarenessContext.Call(dpiAwarenessContextPerMonitorAwareV2)
		if r != 0 {
			return
		}
		if errors.Is(e, windows.ERROR_ACCESS_DENIED) {
			return // awareness was already set (e.g. by a manifest)
		}
		log.Printf("winplat: SetProcessDpiAwarenessContext(per-monitor v2): %v; falling back to SetProcessDPIAware", e)
	}
	if procSetProcessDPIAware.Find() != nil {
		return
	}
	if r, _, e := procSetProcessDPIAware.Call(); r == 0 {
		log.Printf("winplat: SetProcessDPIAware: %v", e)
	}
}
