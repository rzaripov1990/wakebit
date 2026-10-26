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

// titleMatches reports whether a window title refers to the file at path,
// i.e. contains its base name (case-insensitive, as Windows file names are).
// Notepad titles look like "wakebit-time.txt - Notepad" or, when modified,
// "*wakebit-time.txt - Notepad".
func titleMatches(title, path string) bool {
	base := filepath.Base(strings.ReplaceAll(path, `\`, "/"))
	if base == "" || base == "." || base == "/" {
		return false
	}
	return strings.Contains(strings.ToLower(title), strings.ToLower(base))
}

// pickWindow chooses the editor window for path among candidates. A Notepad
// class window wins over any other window whose title merely matches; it
// returns 0 when nothing matches.
func pickWindow(cands []candidate, path string) uintptr {
	var fallback uintptr
	for _, c := range cands {
		if c.hwnd == 0 || !titleMatches(c.title, path) {
			continue
		}
		if strings.EqualFold(c.class, notepadClass) {
			return c.hwnd
		}
		if fallback == 0 {
			fallback = c.hwnd
		}
	}
	return fallback
}

// toAbsolute maps a virtual-desktop pixel coordinate to the 0..65535 range
// SendInput expects with MOUSEEVENTF_ABSOLUTE|MOUSEEVENTF_VIRTUALDESK.
func toAbsolute(v, origin, size int) int32 {
	if size <= 1 {
		return 0
	}
	n := int64(v-origin) * 65535 / int64(size-1)
	if n < 0 {
		n = 0
	}
	if n > 65535 {
		n = 65535
	}
	return int32(n)
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
	for i := 0; i < t.n; i++ {
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
	d := int32(now - t.lastUser)
	if d < 0 {
		d = 0
	}
	return time.Duration(d) * time.Millisecond
}
