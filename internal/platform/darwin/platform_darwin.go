//go:build darwin

package darwinplat

// UNVERIFIED: never compiled or run on a Mac yet (see doc.go).

/*
#cgo LDFLAGS: -framework ApplicationServices -framework CoreGraphics -framework CoreFoundation -framework IOKit
#include <stdbool.h>
#include <CoreFoundation/CoreFoundation.h>
#include <IOKit/pwr_mgt/IOPMLib.h>
#include <ApplicationServices/ApplicationServices.h>

// Every helper takes and returns only plain C values (or pointers to Go
// variables that hold no Go pointers and are not retained), and releases
// every CF object it creates before returning.

// wb_pm_create creates a power assertion: PreventUserIdleDisplaySleep when
// display != 0, otherwise PreventUserIdleSystemSleep. Returns an IOReturn.
static int wb_pm_create(int display, IOPMAssertionID *id) {
	// CFSTR strings (including the kIOPMAssertionType* macros) are
	// compile-time constants and must not be released.
	CFStringRef type = display ? kIOPMAssertionTypePreventUserIdleDisplaySleep
	                           : kIOPMAssertionTypePreventUserIdleSystemSleep;
	return (int)IOPMAssertionCreateWithName(type, kIOPMAssertionLevelOn,
		CFSTR("Wakebit is keeping the computer awake"), id);
}

static int wb_pm_release(IOPMAssertionID id) {
	return (int)IOPMAssertionRelease(id);
}

// wb_cursor stores the cursor position (global display coordinates, points,
// origin top-left). Returns 0 on failure.
static int wb_cursor(double *x, double *y) {
	CGEventRef ev = CGEventCreate(NULL);
	if (ev == NULL) {
		return 0;
	}
	CGPoint p = CGEventGetLocation(ev);
	CFRelease(ev);
	*x = p.x;
	*y = p.y;
	return 1;
}

// wb_warp moves the cursor without generating an event; works without the
// Accessibility permission. Returns a CGError (0 = success).
static int wb_warp(double x, double y) {
	return (int)CGWarpMouseCursorPosition(CGPointMake(x, y));
}

// wb_post_mouse_moved posts a mouse-moved event at (x, y) so the move counts
// as user activity for other apps. Needs Accessibility; returns 0 on failure.
static int wb_post_mouse_moved(double x, double y) {
	CGEventRef ev = CGEventCreateMouseEvent(NULL, kCGEventMouseMoved,
		CGPointMake(x, y), kCGMouseButtonLeft);
	if (ev == NULL) {
		return 0;
	}
	CGEventPost(kCGHIDEventTap, ev);
	CFRelease(ev);
	return 1;
}

static double wb_idle_seconds(void) {
	return CGEventSourceSecondsSinceLastEventType(
		kCGEventSourceStateHIDSystemState, kCGAnyInputEventType);
}

static int wb_ax_trusted(void) {
	return AXIsProcessTrusted() ? 1 : 0;
}

// wb_post_unicode posts a key down/up pair carrying n (1 or 2) UTF-16 units
// (a, b). Modifier flags are cleared so a key the user happens to hold
// (e.g. Cmd) does not turn the character into a shortcut. Returns 0 on
// failure.
static int wb_post_unicode(UniChar a, UniChar b, int n) {
	UniChar units[2];
	units[0] = a;
	units[1] = b;
	CGEventRef down = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)0, true);
	CGEventRef up = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)0, false);
	if (down == NULL || up == NULL) {
		if (down != NULL) {
			CFRelease(down);
		}
		if (up != NULL) {
			CFRelease(up);
		}
		return 0;
	}
	CGEventSetFlags(down, (CGEventFlags)0);
	CGEventSetFlags(up, (CGEventFlags)0);
	CGEventKeyboardSetUnicodeString(down, (UniCharCount)n, units);
	CGEventKeyboardSetUnicodeString(up, (UniCharCount)n, units);
	CGEventPost(kCGHIDEventTap, down);
	CGEventPost(kCGHIDEventTap, up);
	CFRelease(down);
	CFRelease(up);
	return 1;
}
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"wakebit/internal/engine"
)

const (
	osascriptPath = "/usr/bin/osascript"
	openPath      = "/usr/bin/open"

	// scriptTimeout is generous because the first Apple Events call to
	// TextEdit / System Events shows an Automation consent prompt and
	// osascript blocks until the user answers.
	scriptTimeout     = 30 * time.Second
	editorWaitTimeout = 10 * time.Second
)

// Platform is the macOS engine.Platform.
type Platform struct {
	start time.Time // monotonic origin for idle ticks
	idle  idleTracker

	powerMu sync.Mutex
	powerOn bool
	sysID   C.IOPMAssertionID
	dispID  C.IOPMAssertionID

	axOnce sync.Once
}

var _ engine.Platform = (*Platform)(nil)

// New returns the macOS platform.
func New() engine.Platform { return &Platform{start: time.Now()} }

// tick is milliseconds since New on the monotonic clock.
func (p *Platform) tick() int64 { return time.Since(p.start).Milliseconds() }

// PreventSleep creates (on) or releases (off) PreventUserIdleSystemSleep and
// PreventUserIdleDisplaySleep assertions. It is idempotent.
func (p *Platform) PreventSleep(on bool) error {
	p.powerMu.Lock()
	defer p.powerMu.Unlock()
	if on == p.powerOn {
		return nil
	}
	if on {
		var sys, disp C.IOPMAssertionID
		if r := C.wb_pm_create(0, &sys); r != 0 {
			return fmt.Errorf("IOPMAssertionCreateWithName(system): IOReturn 0x%x", uint32(r))
		}
		if r := C.wb_pm_create(1, &disp); r != 0 {
			C.wb_pm_release(sys)
			return fmt.Errorf("IOPMAssertionCreateWithName(display): IOReturn 0x%x", uint32(r))
		}
		p.sysID, p.dispID, p.powerOn = sys, disp, true
		return nil
	}
	var errs []error
	if r := C.wb_pm_release(p.dispID); r != 0 {
		errs = append(errs, fmt.Errorf("IOPMAssertionRelease(display): IOReturn 0x%x", uint32(r)))
	}
	if r := C.wb_pm_release(p.sysID); r != 0 {
		errs = append(errs, fmt.Errorf("IOPMAssertionRelease(system): IOReturn 0x%x", uint32(r)))
	}
	p.sysID, p.dispID, p.powerOn = 0, 0, false
	return errors.Join(errs...)
}

// CursorPos returns the cursor position in global display points.
func (p *Platform) CursorPos() (int, int, error) {
	var x, y C.double
	if C.wb_cursor(&x, &y) == 0 {
		return 0, 0, errors.New("CGEventCreate failed (no window server session?)")
	}
	return int(math.Round(float64(x))), int(math.Round(float64(y))), nil
}

// MoveCursor warps the cursor (no permission needed, so Light mode always
// works). With the Accessibility permission it also posts a mouse-moved
// event, mirroring Windows' SendInput, so other apps see user activity.
func (p *Platform) MoveCursor(x, y int) error {
	p.idle.injected(p.tick())
	defer func() { p.idle.injected(p.tick()) }()
	if r := C.wb_warp(C.double(x), C.double(y)); r != 0 {
		return fmt.Errorf("CGWarpMouseCursorPosition: CGError %d", int(r))
	}
	if C.wb_ax_trusted() != 0 {
		_ = C.wb_post_mouse_moved(C.double(x), C.double(y))
	}
	return nil
}

// IdleFor reports time since the last user input, ignoring our own posted
// events (see idleTracker).
func (p *Platform) IdleFor() time.Duration {
	since := secondsToDuration(float64(C.wb_idle_seconds()))
	return p.idle.idle(p.tick(), since)
}

// checkAccessibility returns ErrNoAccessibility (logging guidance once) when
// the process may not post keyboard events.
func (p *Platform) checkAccessibility() error {
	if C.wb_ax_trusted() != 0 {
		return nil
	}
	p.axOnce.Do(func() {
		log.Printf("wakebit: Full mode needs the Accessibility permission. Open System Settings > Privacy & Security > Accessibility, add/enable Wakebit (or the terminal it was started from), then restart Wakebit. Light mode keeps working meanwhile.")
	})
	return ErrNoAccessibility
}

// runScript runs an AppleScript via osascript and returns its stdout.
func runScript(ctx context.Context, script string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, scriptTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, osascriptPath, scriptArgs(script)...).Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			return "", fmt.Errorf("osascript: %w: %s", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("osascript: %w", err)
	}
	return string(out), nil
}

// findWindowIndex returns the 1-based index of TextEdit's window showing
// path, or 0 when there is none (or TextEdit is not running).
func findWindowIndex(ctx context.Context, path string) (int, error) {
	out, err := runScript(ctx, windowsScript)
	if err != nil {
		return 0, err
	}
	return pickWindowIndex(splitLines(out), path), nil
}

// OpenEditor reuses a TextEdit window showing path, or runs
// `open -a TextEdit path` and waits for the window to appear. It fails early
// without the Accessibility permission so TextEdit is not opened (and focus
// not stolen) when typing would be impossible anyway.
func (p *Platform) OpenEditor(path string) (engine.Editor, error) {
	if err := p.checkAccessibility(); err != nil {
		return nil, err
	}
	ctx := context.Background()
	if i, err := findWindowIndex(ctx, path); err == nil && i != 0 {
		return &editor{p: p, path: path}, nil
	}
	if out, err := exec.Command(openPath, "-a", "TextEdit", path).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("open -a TextEdit: %w: %s", err, strings.TrimSpace(string(out)))
	}
	deadline := time.Now().Add(editorWaitTimeout)
	for time.Now().Before(deadline) {
		if i, err := findWindowIndex(ctx, path); err == nil && i != 0 {
			return &editor{p: p, path: path}, nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return nil, fmt.Errorf("TextEdit window for %s did not appear within %s", filepath.Base(path), editorWaitTimeout)
}

type editor struct {
	p    *Platform
	path string
}

// Focus raises TextEdit's window for our file and activates TextEdit, then
// verifies that it really is frontmost.
func (e *editor) Focus() error {
	ctx := context.Background()
	i, err := findWindowIndex(ctx, e.path)
	if err != nil {
		return fmt.Errorf("list TextEdit windows: %w", err)
	}
	if i == 0 {
		return fmt.Errorf("TextEdit window for %s is gone", filepath.Base(e.path))
	}
	if _, err := runScript(ctx, focusScript(i)); err != nil {
		return fmt.Errorf("focus TextEdit: %w", err)
	}
	for range 10 {
		if out, err := runScript(ctx, frontScript); err == nil && frontIsEditor(out, e.path) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("could not bring the TextEdit window to the front")
}

// Type posts text rune by rune as Unicode key events. Before each rune it
// checks ctx, and that TextEdit is the frontmost app with our file in its
// front window, so keys never land in another app or document.
func (e *editor) Type(ctx context.Context, text string, delay func() time.Duration) error {
	if err := e.p.checkAccessibility(); err != nil {
		return err
	}
	for _, r := range text {
		if err := ctx.Err(); err != nil {
			return err
		}
		out, err := runScript(ctx, frontScript)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("check front window: %w", err)
		}
		if !frontIsEditor(out, e.path) {
			b, w := parseFront(out)
			return fmt.Errorf("TextEdit with %s is no longer in front (front app %q, window %q)", filepath.Base(e.path), b, w)
		}
		a, b, n := runeUnits(r)
		e.p.idle.injected(e.p.tick())
		ok := C.wb_post_unicode(C.UniChar(a), C.UniChar(b), C.int(n))
		e.p.idle.injected(e.p.tick())
		if ok == 0 {
			return errors.New("CGEventCreateKeyboardEvent failed")
		}
		if delay != nil {
			t := time.NewTimer(delay())
			select {
			case <-ctx.Done():
				t.Stop()
				return ctx.Err()
			case <-t.C:
			}
		}
	}
	return nil
}

// Close releases nothing (no handles are held); TextEdit is left open.
func (e *editor) Close() {}
