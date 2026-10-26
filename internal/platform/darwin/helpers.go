package darwinplat

// OS-independent helpers; see doc.go.

import (
	"errors"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

// textEditBundle is TextEdit's bundle identifier (not localized, unlike its
// display name).
const textEditBundle = "com.apple.TextEdit"

// editedSep separates a document name from a status suffix such as
// "— Edited" in macOS window titles. Only the em dash is accepted: a plain
// " - " is common in real file names ("x - copy.txt").
const editedSep = " — "

// windowNameMatches reports whether a TextEdit window name refers to the
// file at path: exactly the file's base name, or the base name without its
// extension (Finder may hide extensions), optionally followed by an em-dash
// status suffix ("wakebit-time.txt — Edited"). Comparison is
// case-insensitive, as default macOS volumes are.
func windowNameMatches(name, path string) bool {
	base := filepath.Base(path)
	if base == "" || base == "." || base == "/" {
		return false
	}
	if i := strings.Index(name, editedSep); i >= 0 {
		if strings.TrimSpace(name[i+len(editedSep):]) == "" {
			return false
		}
		name = name[:i]
	}
	if strings.EqualFold(name, base) {
		return true
	}
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	return stem != "" && stem != base && strings.EqualFold(name, stem)
}

// pickWindowIndex returns the 1-based AppleScript index of the first window
// name that matches path, or 0.
func pickWindowIndex(names []string, path string) int {
	for i, n := range names {
		if windowNameMatches(n, path) {
			return i + 1
		}
	}
	return 0
}

// splitLines splits osascript output into lines, dropping trailing blank
// lines and CRs but keeping blank lines in the middle (their position is a
// window index).
func splitLines(out string) []string {
	out = strings.TrimRight(out, "\r\n")
	if out == "" {
		return nil
	}
	lines := strings.Split(out, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSuffix(l, "\r")
	}
	return lines
}

// windowsScript prints the names of TextEdit's windows, front to back, one
// per line. It prints nothing (and does not launch TextEdit) when TextEdit
// is not running.
const windowsScript = `set out to ""
if application "TextEdit" is running then
	tell application "TextEdit"
		repeat with w in windows
			set out to out & (name of w) & linefeed
		end repeat
	end tell
end if
return out`

// frontScript prints the frontmost application's bundle identifier and, only
// when that is TextEdit, the name of TextEdit's front window on a second
// line. It fails (non-zero exit) if TextEdit has no window.
const frontScript = `tell application "System Events" to set fb to bundle identifier of first application process whose frontmost is true
if fb is not "com.apple.TextEdit" then return (fb as text)
tell application "TextEdit" to set wn to name of window 1
return fb & linefeed & wn`

// focusScript raises TextEdit's window number index (1-based, from
// windowsScript) to the front and activates TextEdit. index is an int, so
// nothing user-controlled is ever spliced into the script.
func focusScript(index int) string {
	return `tell application "TextEdit"
	set index of window ` + strconv.Itoa(index) + ` to 1
	activate
end tell`
}

// scriptArgs turns a multi-line AppleScript into osascript arguments, one
// -e per line (the conventional, quoting-free way to pass a script).
func scriptArgs(script string) []string {
	var args []string
	for _, l := range strings.Split(script, "\n") {
		args = append(args, "-e", l)
	}
	return args
}

// parseFront splits frontScript output into bundle id and window name.
func parseFront(out string) (bundle, window string) {
	lines := splitLines(out)
	if len(lines) > 0 {
		bundle = strings.TrimSpace(lines[0])
	}
	if len(lines) > 1 {
		window = lines[1]
	}
	return bundle, window
}

// frontIsEditor reports whether frontScript output says TextEdit is the
// frontmost app and its front window shows path.
func frontIsEditor(out, path string) bool {
	b, w := parseFront(out)
	return b == textEditBundle && windowNameMatches(w, path)
}

// runeUnits returns r as one or two UTF-16 code units (n = count). Invalid
// runes (e.g. lone surrogates) become U+FFFD.
func runeUnits(r rune) (a, b uint16, n int) {
	if !utf8.ValidRune(r) {
		r = utf8.RuneError
	}
	if r >= 0x10000 {
		hi, lo := utf16.EncodeRune(r)
		return uint16(hi), uint16(lo), 2
	}
	return uint16(r), 0, 1
}

// maxIdle caps idle durations so float->Duration conversion cannot overflow.
const maxIdle = 100 * 365 * 24 * time.Hour

// secondsToDuration converts CGEventSourceSecondsSinceLastEventType's result
// to a Duration: negative/NaN -> 0, huge/Inf -> maxIdle.
func secondsToDuration(s float64) time.Duration {
	if math.IsNaN(s) || s <= 0 {
		return 0
	}
	if s >= maxIdle.Seconds() {
		return maxIdle
	}
	return time.Duration(s * float64(time.Second))
}

// injectTolerance is how close (ms) the system's last-input time must be to
// one of our own injections for it to be attributed to us.
const injectTolerance = 100

// injectRing is how many recent own-injection times are remembered.
const injectRing = 256

// idleTracker estimates time since the last *real* user input. It mirrors
// the Windows tracker: CGEventSourceSecondsSinceLastEventType (HID system
// state) also counts events we post ourselves, so we record our injection
// times and, when the reported last-input time lies within injectTolerance
// of one of them, keep the previously known user-input time. Times are
// milliseconds on a monotonic clock (see Platform.tick); user input that
// happens during one of our bursts can be masked until the next real input.
type idleTracker struct {
	mu       sync.Mutex
	ticks    [injectRing]int64
	n        int
	next     int
	lastUser int64
	haveUser bool
}

// injected records that we injected input at tick now (ms).
func (t *idleTracker) injected(now int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ticks[t.next] = now
	t.next = (t.next + 1) % injectRing
	if t.n < injectRing {
		t.n++
	}
}

func (t *idleTracker) isOwnLocked(tick int64) bool {
	for i := range t.n {
		d := tick - t.ticks[i]
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
// (ms) and the system's time since its last input event.
func (t *idleTracker) idle(now int64, since time.Duration) time.Duration {
	lastInput := now - since.Milliseconds()
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.haveUser || !t.isOwnLocked(lastInput) {
		if !t.haveUser || lastInput > t.lastUser {
			t.lastUser = lastInput
		}
		t.haveUser = true
	}
	return time.Duration(max(now-t.lastUser, 0)) * time.Millisecond
}

// ErrNoAccessibility is returned by the editor when the process is not
// trusted for Accessibility, without which macOS drops synthetic key events.
var ErrNoAccessibility = errors.New("macOS Accessibility permission not granted: allow Wakebit in System Settings > Privacy & Security > Accessibility (Full mode needs it to type; Light mode works without it)")
