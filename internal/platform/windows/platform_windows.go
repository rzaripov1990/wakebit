package winplat

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"

	"wakebit/internal/engine"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procSetCursorPos        = user32.NewProc("SetCursorPos")
	procSendInput           = user32.NewProc("SendInput")
	procGetLastInputInfo    = user32.NewProc("GetLastInputInfo")
	procGetWindowTextW      = user32.NewProc("GetWindowTextW")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procBringWindowToTop    = user32.NewProc("BringWindowToTop")
	procShowWindow          = user32.NewProc("ShowWindow")
	procShowWindowAsync     = user32.NewProc("ShowWindowAsync")
	procIsIconic            = user32.NewProc("IsIconic")
	procGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	procAttachThreadInput   = user32.NewProc("AttachThreadInput")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procFindWindowExW       = user32.NewProc("FindWindowExW")
	procGetAsyncKeyState    = user32.NewProc("GetAsyncKeyState")

	procPowerCreateRequest = kernel32.NewProc("PowerCreateRequest")
	procPowerSetRequest    = kernel32.NewProc("PowerSetRequest")
	procPowerClearRequest  = kernel32.NewProc("PowerClearRequest")
)

const (
	powerRequestDisplayRequired = 0
	powerRequestSystemRequired  = 1

	powerRequestContextVersion      = 0
	powerRequestContextSimpleString = 0x1

	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCXVirtualScreen = 78
	smCYVirtualScreen = 79

	swRestore = 9

	wmClose = 0x0010

	editorWaitTimeout = 10 * time.Second

	// quitTimeout is Quit's single overall deadline (foreground, quiet
	// period, save, close, wait); Stop waits for it on the tray thread.
	quitTimeout = 3 * time.Second
	// quitForegroundWait caps how long Quit waits for Notepad to become the
	// foreground window before giving up.
	quitForegroundWait = 500 * time.Millisecond
)

// reasonContext mirrors REASON_CONTEXT with the SimpleReasonString variant.
// Reason overlays the union's first field (HMODULE); the padding fills the
// rest of its larger Detailed variant (ULONG, ULONG, LPWSTR*). Size is 32
// bytes on 64-bit and 24 on 32-bit.
type reasonContext struct {
	Version uint32
	Flags   uint32
	Reason  *uint16
	_       [8]byte
	_       uintptr
}

type point struct{ X, Y int32 }

type lastInputInfo struct {
	CbSize uint32
	DwTime uint32
}

// Platform is the Windows engine.Platform.
type Platform struct {
	idle idleTracker

	powerMu  sync.Mutex
	powerReq windows.Handle
	powerOn  bool
	reason   *uint16 // kept alive for the lifetime of powerReq
	// powerReq is created once and intentionally never closed: it lives for
	// the whole process and the OS releases it on exit.
}

var (
	_ engine.Platform = (*Platform)(nil)
	_ engine.Quitter  = (*editor)(nil)
)

// New returns the Windows platform. It also makes the process DPI-aware
// (see EnsureDPIAware) as a safety net for callers that did not do so.
func New() engine.Platform {
	EnsureDPIAware()
	return &Platform{}
}

func tickNow() uint32 { return uint32(windows.DurationSinceBoot() / time.Millisecond) }

// PreventSleep sets (on) or clears (off) system- and display-required power
// requests. Power requests are owned by a handle, not a thread, so this is
// safe from any goroutine. It is idempotent.
func (p *Platform) PreventSleep(on bool) error {
	p.powerMu.Lock()
	defer p.powerMu.Unlock()
	if on == p.powerOn {
		return nil
	}
	if on {
		if p.powerReq == 0 {
			reason, err := windows.UTF16PtrFromString("Wakebit is keeping the computer awake")
			if err != nil {
				return err
			}
			rc := reasonContext{Version: powerRequestContextVersion, Flags: powerRequestContextSimpleString, Reason: reason}
			h, _, e := procPowerCreateRequest.Call(uintptr(unsafe.Pointer(&rc)))
			if windows.Handle(h) == windows.InvalidHandle || h == 0 {
				return fmt.Errorf("PowerCreateRequest: %w", e)
			}
			p.powerReq, p.reason = windows.Handle(h), reason
		}
		if r, _, e := procPowerSetRequest.Call(uintptr(p.powerReq), powerRequestSystemRequired); r == 0 {
			return fmt.Errorf("PowerSetRequest(system): %w", e)
		}
		if r, _, e := procPowerSetRequest.Call(uintptr(p.powerReq), powerRequestDisplayRequired); r == 0 {
			procPowerClearRequest.Call(uintptr(p.powerReq), powerRequestSystemRequired)
			return fmt.Errorf("PowerSetRequest(display): %w", e)
		}
		p.powerOn = true
		return nil
	}
	var errs []error
	if r, _, e := procPowerClearRequest.Call(uintptr(p.powerReq), powerRequestDisplayRequired); r == 0 {
		errs = append(errs, fmt.Errorf("PowerClearRequest(display): %w", e))
	}
	if r, _, e := procPowerClearRequest.Call(uintptr(p.powerReq), powerRequestSystemRequired); r == 0 {
		errs = append(errs, fmt.Errorf("PowerClearRequest(system): %w", e))
	}
	p.powerOn = false
	return errors.Join(errs...)
}

// CursorPos returns the cursor position in virtual-desktop pixels.
func (p *Platform) CursorPos() (int, int, error) {
	var pt point
	if r, _, e := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt))); r == 0 {
		return 0, 0, fmt.Errorf("GetCursorPos: %w", e)
	}
	return int(pt.X), int(pt.Y), nil
}

// MoveCursor moves the cursor via SendInput (so the move counts as input for
// presence/idle detection in other apps) and then pins the exact pixel with
// SetCursorPos, since absolute SendInput coordinates are rounded.
func (p *Platform) MoveCursor(x, y int) error {
	ox := sysMetric(smXVirtualScreen)
	oy := sysMetric(smYVirtualScreen)
	w := sysMetric(smCXVirtualScreen)
	h := sysMetric(smCYVirtualScreen)
	in := mouseINPUT{Type: inputMouse, Mi: mouseInput{
		Dx:    toAbsolute(x, ox, w),
		Dy:    toAbsolute(y, oy, h),
		Flags: mouseeventfMove | mouseeventfAbsolute | mouseeventfVirtualDesk,
	}}
	p.idle.injected(tickNow())
	r, _, e := procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
	p.idle.injected(tickNow())
	if r == 0 {
		return fmt.Errorf("SendInput(mouse): %w", e)
	}
	if r, _, e := procSetCursorPos.Call(uintptr(int32(x)), uintptr(int32(y))); r == 0 {
		return fmt.Errorf("SetCursorPos: %w", e)
	}
	return nil
}

func sysMetric(i int) int {
	r, _, _ := procGetSystemMetrics.Call(uintptr(i))
	return int(int32(r))
}

// IdleFor reports time since the last user input, ignoring our own
// injections (see idleTracker).
func (p *Platform) IdleFor() time.Duration {
	li := lastInputInfo{CbSize: uint32(unsafe.Sizeof(lastInputInfo{}))}
	if r, _, _ := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&li))); r == 0 {
		// Unknown: report "not idle" so the engine stays conservative.
		return 0
	}
	return p.idle.idle(tickNow(), li.DwTime)
}

// sendKeys injects keyboard INPUTs and records them as our own input.
func (p *Platform) sendKeys(ins []keybdINPUT) error {
	if len(ins) == 0 {
		return nil
	}
	p.idle.injected(tickNow())
	r, _, e := procSendInput.Call(uintptr(len(ins)), uintptr(unsafe.Pointer(&ins[0])), unsafe.Sizeof(ins[0]))
	p.idle.injected(tickNow())
	if int(r) != len(ins) {
		return fmt.Errorf("SendInput(keyboard): sent %d of %d: %w", r, len(ins), e)
	}
	return nil
}

// OpenEditor reuses an existing Notepad window showing path, or starts
// notepad.exe and waits for its window to appear.
func (p *Platform) OpenEditor(path string) (engine.Editor, error) {
	if h := findWindow(path); h != 0 {
		return &editor{p: p, path: path, hwnd: h}, nil
	}
	sysDir, err := windows.GetSystemDirectory()
	if err != nil {
		return nil, fmt.Errorf("system directory: %w", err)
	}
	// Absolute path: never pick up a notepad.exe from PATH or the cwd.
	cmd := exec.Command(filepath.Join(sysDir, "notepad.exe"), path)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start notepad: %w", err)
	}
	// Notepad may hand off to another process (Windows 11), so we find the
	// window by title rather than by PID; reap the child in the background.
	go func() { _ = cmd.Wait() }()
	deadline := time.Now().Add(editorWaitTimeout)
	for time.Now().Before(deadline) {
		if h := findWindow(path); h != 0 {
			return &editor{p: p, path: path, hwnd: h}, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil, fmt.Errorf("notepad window for %s did not appear within %s (notepad-like windows seen: %s)",
		filepath.Base(path), editorWaitTimeout, describeNotepadLike())
}

// describeNotepadLike lists the visible windows that look like Notepad, as
// "class=... title=..." pairs, to explain why title matching found nothing.
func describeNotepadLike() string {
	enumMu.Lock()
	defer enumMu.Unlock()
	enumCands = enumCands[:0]
	_ = windows.EnumWindows(enumCB, nil)
	var out []string
	for _, c := range enumCands {
		l := strings.ToLower(c.title)
		if strings.EqualFold(c.class, notepadClass) || strings.Contains(l, "wakebit") ||
			strings.Contains(l, "notepad") || strings.Contains(l, "блокнот") {
			out = append(out, fmt.Sprintf("[class=%q title=%q]", c.class, c.title))
		}
	}
	if len(out) == 0 {
		return "none"
	}
	return strings.Join(out, " ")
}

// Window enumeration uses one package-level callback (Go limits the number
// of callbacks created by NewCallback) and a mutex-guarded result slice.
var (
	enumMu    sync.Mutex
	enumCands []candidate
	enumCB    = windows.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		if !windows.IsWindowVisible(hwnd) {
			return 1
		}
		title := windowText(hwnd)
		if title == "" {
			return 1
		}
		enumCands = append(enumCands, candidate{hwnd: uintptr(hwnd), title: title, class: windowClass(hwnd)})
		return 1
	})
)

func windowText(hwnd windows.HWND) string {
	var buf [512]uint16
	n, _, _ := procGetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return windows.UTF16ToString(buf[:n])
}

func windowClass(hwnd windows.HWND) string {
	var cls [256]uint16
	n, _ := windows.GetClassName(hwnd, &cls[0], int32(len(cls)))
	return windows.UTF16ToString(cls[:n])
}

// isEditorWindow reports whether hwnd is (still) a Notepad window whose
// active document is path.
func isEditorWindow(hwnd windows.HWND, path string) bool {
	return hwnd != 0 && windows.IsWindow(hwnd) &&
		pickWindow([]candidate{{hwnd: uintptr(hwnd), title: windowText(hwnd), class: windowClass(hwnd)}}, path) != 0
}

// EditorWindowOpen reports whether a Notepad window showing path exists
// (same strict class and title match the editor uses). For cmd/selftest.
func EditorWindowOpen(path string) bool { return findWindow(path) != 0 }

func findWindow(path string) windows.HWND {
	enumMu.Lock()
	defer enumMu.Unlock()
	enumCands = enumCands[:0]
	_ = windows.EnumWindows(enumCB, nil)
	return windows.HWND(pickWindow(enumCands, path))
}

type editor struct {
	p    *Platform
	path string
	hwnd windows.HWND
}

// Focus re-locates the window if needed and brings it to the foreground.
func (e *editor) Focus() error {
	if !isEditorWindow(e.hwnd, e.path) {
		e.hwnd = findWindow(e.path)
		if e.hwnd == 0 {
			return fmt.Errorf("editor window for %s: %w", filepath.Base(e.path), engine.ErrEditorGone)
		}
	}
	if r, _, _ := procIsIconic.Call(uintptr(e.hwnd)); r != 0 {
		procShowWindow.Call(uintptr(e.hwnd), swRestore)
	}
	if e.tryForeground() {
		return nil
	}
	if e.attachForeground() {
		return nil
	}
	// Last resort: a background process may only take the foreground right
	// after input; a synthetic ALT tap satisfies that rule. SendInput only
	// queues events, so give them time to be processed before retrying.
	if e.altTap() == nil {
		time.Sleep(50 * time.Millisecond)
		if e.tryForeground() {
			return nil
		}
		// The tap may have left the still-foreground app in menu mode. A
		// second ALT tap toggles menu mode off again; we prefer it over Esc,
		// which could dismiss a dialog in that app.
		if windows.GetForegroundWindow() != e.hwnd {
			_ = e.altTap()
		}
	}
	return errors.New("could not bring editor window to the foreground")
}

func (e *editor) altTap() error {
	return e.p.sendKeys([]keybdINPUT{
		{Type: inputKeyboard, Ki: keybdInput{Vk: vkMenu}},
		{Type: inputKeyboard, Ki: keybdInput{Vk: vkMenu, Flags: keyeventfKeyUp}},
	})
}

func (e *editor) tryForeground() bool {
	procSetForegroundWindow.Call(uintptr(e.hwnd))
	return waitForeground(e.hwnd)
}

// attachForeground temporarily joins the current foreground thread's input
// queue, which lets SetForegroundWindow succeed from a background process.
func (e *editor) attachForeground() bool {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	fg := windows.GetForegroundWindow()
	if fg == 0 {
		return false
	}
	fgTID, _ := windows.GetWindowThreadProcessId(fg, nil)
	me := windows.GetCurrentThreadId()
	if fgTID == 0 || fgTID == me {
		return false
	}
	if r, _, _ := procAttachThreadInput.Call(uintptr(me), uintptr(fgTID), 1); r == 0 {
		return false
	}
	procBringWindowToTop.Call(uintptr(e.hwnd))
	procSetForegroundWindow.Call(uintptr(e.hwnd))
	procAttachThreadInput.Call(uintptr(me), uintptr(fgTID), 0)
	return waitForeground(e.hwnd)
}

func waitForeground(h windows.HWND) bool {
	for range 10 {
		if windows.GetForegroundWindow() == h {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// Type sends text rune by rune with KEYEVENTF_UNICODE. Before each rune it
// checks ctx, that the editor is still the foreground window, and that its
// title still names our file (a Windows 11 Notepad tab switch keeps the same
// HWND), so keys never land in another app or document.
func (e *editor) Type(ctx context.Context, text string, delay func() time.Duration) error {
	for _, r := range text {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !windows.IsWindow(e.hwnd) {
			return fmt.Errorf("editor window for %s: %w", filepath.Base(e.path), engine.ErrEditorGone)
		}
		if fg := windows.GetForegroundWindow(); fg != e.hwnd {
			return fmt.Errorf("editor lost focus (foreground is %#x)", uintptr(fg))
		}
		if title := windowText(e.hwnd); !titleMatches(title, e.path) {
			return fmt.Errorf("editor no longer shows %s (title %q)", filepath.Base(e.path), title)
		}
		if err := e.p.sendKeys(unicodeKeys(utf16.Encode([]rune{r}))); err != nil {
			return err
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

// Close releases handles only; the Notepad window is left open (Quit
// closes it).
func (e *editor) Close() { e.hwnd = 0 }

// Quit saves our temp file (Ctrl+S) and closes it so no "save changes?"
// prompt appears: classic Notepad gets WM_CLOSE, tabbed Windows 11 Notepad
// gets Ctrl+W so only our tab closes (see quitMethod).
//
// Safety rules:
//   - It acts only on the HWND the editor already holds and never searches
//     for another window; a window that is already gone is success.
//   - Before each input (Ctrl+S, then WM_CLOSE or Ctrl+W) it waits for the
//     user to have been idle for quitQuiet and then re-checks that the HWND
//     is a "Notepad"-class window whose title names our file, that it is the
//     foreground window and that no modifier key is down.
//   - It closes only after the title has lost its "*" (file saved); if the
//     save does not complete (read-only file, Save As or error dialog) the
//     window and any dialog are left alone.
//   - Every wait shares one deadline (quitTimeout); it never blocks on the
//     Notepad process (only asynchronous Win32 calls) and never kills it.
//
// It returns an error, leaving the window open, whenever a check fails.
func (e *editor) Quit() error {
	p := newQuitPacer(quitTimeout)
	h, path, name := e.hwnd, e.path, filepath.Base(e.path)
	if h == 0 || !windows.IsWindow(h) {
		return nil
	}
	if !isEditorWindow(h, path) {
		return fmt.Errorf("notepad window no longer shows %s; left open", name)
	}
	if err := quitForeground(h, path, p); err != nil {
		return err
	}
	ready := func() error {
		if modifiersHeld() {
			return errors.New("a modifier key is held; notepad left open")
		}
		if !isEditorWindow(h, path) {
			return fmt.Errorf("notepad window no longer shows %s; left open", name)
		}
		if fg := windows.GetForegroundWindow(); fg != h {
			return fmt.Errorf("notepad lost focus (foreground is %#x); left open", uintptr(fg))
		}
		return nil
	}
	if err := waitQuiet(p, e.p.IdleFor, ready); err != nil {
		return fmt.Errorf("before save: %w", err)
	}
	if err := e.p.sendKeys(ctrlChord('S')); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	read := func() winState {
		if !windows.IsWindow(h) {
			return winState{}
		}
		return winState{alive: true, title: windowText(h)}
	}
	if err := waitSaved(p, read, path); err != nil {
		if errors.Is(err, errQuitWindowGone) {
			return nil
		}
		return err
	}
	how := quitMethod(hasChildClass(h, "Edit"))
	if err := waitQuiet(p, e.p.IdleFor, ready); err != nil {
		return fmt.Errorf("before close: %w", err)
	}
	switch how {
	case quitByWMClose:
		// PostMessage, not SendMessage: a hung Notepad cannot block us.
		if r, _, err := procPostMessageW.Call(uintptr(h), wmClose, 0, 0); r == 0 {
			return fmt.Errorf("PostMessage(WM_CLOSE): %w", err)
		}
	default:
		if err := e.p.sendKeys(ctrlChord('W')); err != nil {
			return fmt.Errorf("close tab: %w", err)
		}
	}
	return waitClosed(p, read, path, func() bool { return findWindow(path) != 0 })
}

// quitForeground brings the held window h to the foreground for Quit,
// gently: no AttachThreadInput, no synthetic ALT tap, and only asynchronous
// calls (ShowWindowAsync rather than ShowWindow; no BringWindowToTop, which
// is a synchronous SetWindowPos into Notepad's thread and could block on a
// hung Notepad). It never looks for another window. If h is not the
// foreground window within quitForegroundWait (capped by p) it refuses.
func quitForeground(h windows.HWND, path string, p quitPacer) error {
	if !isEditorWindow(h, path) {
		return fmt.Errorf("notepad window no longer shows %s; left open", filepath.Base(path))
	}
	if windows.GetForegroundWindow() == h {
		return nil
	}
	if r, _, _ := procIsIconic.Call(uintptr(h)); r != 0 {
		procShowWindowAsync.Call(uintptr(h), swRestore)
	}
	procSetForegroundWindow.Call(uintptr(h))
	w := p.within(quitForegroundWait)
	for windows.GetForegroundWindow() != h {
		if !w.wait() {
			return errors.New("could not bring notepad to the foreground; left open")
		}
	}
	if !isEditorWindow(h, path) {
		return fmt.Errorf("notepad window no longer shows %s; left open", filepath.Base(path))
	}
	return nil
}

// modifiersHeld reports whether Shift, Ctrl, Alt or Win is down.
// GetAsyncKeyState also reflects keys injected with SendInput (by us or any
// other app), not only physical keys; ongoing physical typing is covered by
// the quiet-period check (waitQuiet), not by this.
func modifiersHeld() bool {
	var st []uint16
	for _, vk := range []uintptr{vkShift, vkControl, vkMenu, vkLWin, vkRWin} {
		r, _, _ := procGetAsyncKeyState.Call(vk)
		st = append(st, uint16(r))
	}
	return modifierHeld(st)
}

// hasChildClass reports whether h has a direct child window of class cls.
func hasChildClass(h windows.HWND, cls string) bool {
	p, err := windows.UTF16PtrFromString(cls)
	if err != nil {
		return false
	}
	r, _, _ := procFindWindowExW.Call(uintptr(h), 0, uintptr(unsafe.Pointer(p)), 0)
	return r != 0
}
