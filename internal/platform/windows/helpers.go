// Package winplat implements engine.Platform on Windows.
//
// This file holds pure, OS-independent helpers so they can be unit-tested on
// any GOOS; the Win32 glue lives in platform_windows.go.
package winplat

import (
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// candidate is a top-level window seen during enumeration.
type candidate struct {
	hwnd  uintptr
	title string
	class string
}

// notepadClass is the window class of classic and Windows 11 Notepad.
const notepadClass = "Notepad"

// titleMatches reports whether a Notepad window title refers to the file at
// path: an optional leading "*" (unsaved changes), then exactly the file's
// base name, or the base name without its extension (Explorer's "hide
// extensions for known file types" makes Notepad show the stem only), then
// " - ". Comparison is case-insensitive, as Windows file names are. Whatever
// follows (the localized app name) is ignored. Examples that match:
// "wakebit-time.txt - Notepad", "*wakebit-time.txt - Блокнот",
// "wakebit-time - Notepad". "wakebit-time.bak - Notepad" does not.
func titleMatches(title, path string) bool {
	base := filepath.Base(strings.ReplaceAll(path, `\`, "/"))
	if base == "" || base == "." || base == "/" {
		return false
	}
	t := strings.TrimPrefix(title, "*")
	hasPrefix := func(name string) bool {
		prefix := name + " - "
		return len(t) >= len(prefix) && strings.EqualFold(t[:len(prefix)], prefix)
	}
	if hasPrefix(base) {
		return true
	}
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	return stem != "" && stem != base && hasPrefix(stem)
}

// pickWindow returns the Notepad window showing path, or 0. Windows of any
// other class are never chosen, even when their title matches, so keystrokes
// cannot land in another application.
func pickWindow(cands []candidate, path string) uintptr {
	for _, c := range cands {
		if c.hwnd != 0 && strings.EqualFold(c.class, notepadClass) && titleMatches(c.title, path) {
			return c.hwnd
		}
	}
	return 0
}

// toAbsolute maps a virtual-desktop pixel coordinate to the 0..65535 range
// SendInput expects with MOUSEEVENTF_ABSOLUTE|MOUSEEVENTF_VIRTUALDESK.
func toAbsolute(v, origin, size int) int32 {
	if size <= 1 {
		return 0
	}
	n := int64(v-origin) * 65535 / int64(size-1)
	return int32(min(max(n, 0), 65535))
}

// injectTolerance is how close (ms) the system's last-input tick must be to
// one of our own injection ticks for it to be attributed to us.
const injectTolerance = 100

// injectRing is how many recent own-injection ticks are remembered.
const injectRing = 256

// idleTracker estimates time since the last *real* user input.
//
// GetLastInputInfo cannot tell our own SendInput events from the user's, so
// we record the GetTickCount value at each own injection. When the system's
// last-input tick lies within injectTolerance ms of a recorded injection, we
// treat it as ours and keep the previously known user-input tick; otherwise
// the new tick is adopted as the latest user input. This is pragmatic: user
// input that happens *during* one of our bursts and is followed by another of
// our injections before IdleFor is polled gets masked, but the next real input
// after the burst is detected normally. All tick math is uint32 wrap-safe.
type idleTracker struct {
	mu       sync.Mutex
	ticks    [injectRing]uint32
	n        int // number of valid entries (<= injectRing)
	next     int // ring write index
	lastUser uint32
	haveUser bool
}

// injected records that we injected input at tick now.
func (t *idleTracker) injected(now uint32) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ticks[t.next] = now
	t.next = (t.next + 1) % injectRing
	if t.n < injectRing {
		t.n++
	}
}

func (t *idleTracker) isOwnLocked(tick uint32) bool {
	for i := range t.n {
		d := int32(tick - t.ticks[i])
		if d < 0 {
			d = -d
		}
		if d <= injectTolerance {
			return true
		}
	}
	return false
}

// idle returns the time since the last user input, given the current tick
// and the system's last-input tick.
func (t *idleTracker) idle(now, lastInput uint32) time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.haveUser || !t.isOwnLocked(lastInput) {
		// Only move forward: an older tick never replaces a newer one.
		if !t.haveUser || int32(lastInput-t.lastUser) > 0 {
			t.lastUser = lastInput
		}
		t.haveUser = true
	}
	d := max(int32(now-t.lastUser), 0)
	return time.Duration(d) * time.Millisecond
}

// quitHow is how Quit closes the Notepad window showing our file.
type quitHow int

const (
	// quitByWMClose posts WM_CLOSE: classic Notepad holds one document per
	// window, so closing the window closes only our file.
	quitByWMClose quitHow = iota
	// quitByCloseTab sends Ctrl+W: Windows 11 Notepad may have opened our
	// file as a tab in a window that also holds the user's documents, and
	// WM_CLOSE would close (or prompt for) all of them. Ctrl+W closes only
	// the active tab, which Quit has verified is ours; closing the last tab
	// closes the window.
	quitByCloseTab
)

// quitMethod picks the close method: classic Notepad has a direct "Edit"
// child window, the tabbed Windows 11 Notepad does not.
func quitMethod(hasEditChild bool) quitHow {
	if hasEditChild {
		return quitByWMClose
	}
	return quitByCloseTab
}

// quitDone reports whether our document is closed: the window is gone, or
// (tabbed Notepad) the window now shows another tab.
func quitDone(alive bool, title, path string, how quitHow) bool {
	if !alive {
		return true
	}
	return how == quitByCloseTab && !titleMatches(title, path)
}

// modifierHeld reports whether any GetAsyncKeyState result has the "key is
// down" (high) bit set. A physically held Shift/Alt/Win would turn our
// Ctrl+S into another shortcut (Ctrl+Shift+S is "Save as").
func modifierHeld(states []uint16) bool {
	for _, s := range states {
		if s&0x8000 != 0 {
			return true
		}
	}
	return false
}
