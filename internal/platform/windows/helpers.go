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
// base name (case-insensitive, as Windows file names are), then " - ".
// Whatever follows (the localized app name) is ignored. Examples that match:
// "wakebit-time.txt - Notepad", "*wakebit-time.txt - Блокнот".
func titleMatches(title, path string) bool {
	base := filepath.Base(strings.ReplaceAll(path, `\`, "/"))
	if base == "" || base == "." || base == "/" {
		return false
	}
	t := strings.TrimPrefix(title, "*")
	prefix := base + " - "
	return len(t) >= len(prefix) && strings.EqualFold(t[:len(prefix)], prefix)
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
