package winplat

import (
	"errors"
	"strings"
	"testing"
	"time"
	"unsafe"
)

func TestTitleMatches(t *testing.T) {
	p := `C:\Users\me\AppData\Local\Temp\wakebit-time.txt`
	cases := []struct {
		title string
		want  bool
	}{
		{"wakebit-time.txt - Notepad", true},
		{"wakebit-time.txt — Блокнот", true},
		{"*wakebit-time.txt – Notepad", true},
		{"wakebit-time — Блокнот", true},
		{"wakebit-time.bak — Блокнот", false},
		{"*wakebit-time.txt - Notepad", true},
		{"WAKEBIT-TIME.TXT - Блокнот", true},
		// Explorer "hide extensions for known file types": stem only.
		{"wakebit-time - Notepad", true},
		{"*wakebit-time - Notepad", true},
		{"WAKEBIT-TIME - Блокнот", true},
		{"old-wakebit-time - Notepad", false},
		{"wakebit-time.bak - Notepad", false},
		{"wakebit-times - Notepad", false},
		{"**wakebit-time - Notepad", false},
		{"wakebit-time", false},
		{"wakebit-time -", false},
		{"old-wakebit-time.txt - Notepad", false},
		{"wakebit-time.txt.bak - Notepad", false},
		{"**wakebit-time.txt - Notepad", false},
		{"wakebit-time.txt", false},
		{"wakebit-time.txt -", false},
		{"Untitled - Notepad", false},
		{"", false},
	}
	for _, c := range cases {
		if got := titleMatches(c.title, p); got != c.want {
			t.Errorf("titleMatches(%q) = %v, want %v", c.title, got, c.want)
		}
	}
	if titleMatches("anything", "") {
		t.Error("empty path must not match")
	}
}

func TestPickWindowNotepadOnly(t *testing.T) {
	p := "/tmp/wakebit-time.txt"
	cands := []candidate{
		{hwnd: 1, title: "Untitled - Notepad", class: "Notepad"},
		{hwnd: 2, title: "wakebit-time.txt - Visual Studio Code", class: "Chrome_WidgetWin_1"},
		{hwnd: 3, title: "wakebit-time.txt - Notepad++", class: "Notepad++"},
		{hwnd: 4, title: "old-wakebit-time.txt - Notepad", class: "Notepad"},
		{hwnd: 5, title: "wakebit-time.txt.bak - Notepad", class: "Notepad"},
		{hwnd: 7, title: "wakebit-time - Visual Studio Code", class: "Chrome_WidgetWin_1"},
		{hwnd: 8, title: "wakebit-time.bak - Notepad", class: "Notepad"},
		{hwnd: 6, title: "*wakebit-time.txt - Notepad", class: "Notepad"},
	}
	if got := pickWindow(cands, p); got != 6 {
		t.Fatalf("got %d, want 6", got)
	}
	// Same base name in another app, or a different file in Notepad: rejected.
	if got := pickWindow(cands[:7], p); got != 0 {
		t.Fatalf("non-Notepad or wrong file accepted: got %d, want 0", got)
	}
	// Hidden-extension title in Notepad is accepted.
	hidden := append(cands[:7:7], candidate{hwnd: 9, title: "*wakebit-time - Notepad", class: "Notepad"})
	if got := pickWindow(hidden, p); got != 9 {
		t.Fatalf("hidden-extension title: got %d, want 9", got)
	}
	if got := pickWindow(cands[:5], p); got != 0 {
		t.Fatalf("non-Notepad or wrong file accepted: got %d, want 0", got)
	}
}

func TestToAbsolute(t *testing.T) {
	if got := toAbsolute(0, 0, 1920); got != 0 {
		t.Errorf("left edge: %d", got)
	}
	if got := toAbsolute(1919, 0, 1920); got != 65535 {
		t.Errorf("right edge: %d", got)
	}
	if got := toAbsolute(-1920, -1920, 3840); got != 0 {
		t.Errorf("negative origin: %d", got)
	}
	if got := toAbsolute(5000, 0, 1920); got != 65535 {
		t.Errorf("clamp: %d", got)
	}
	if got := toAbsolute(10, 0, 1); got != 0 {
		t.Errorf("degenerate size: %d", got)
	}
}

func TestIdleTrackerIgnoresOwnInput(t *testing.T) {
	var tr idleTracker
	// User input at tick 1000; now 7000 -> idle 6s.
	if got := tr.idle(7000, 1000); got != 6*time.Second {
		t.Fatalf("initial: %v", got)
	}
	// We inject at 7000; system last input becomes 7010 (ours, within tol).
	tr.injected(7000)
	if got := tr.idle(8000, 7010); got != 7*time.Second {
		t.Fatalf("own input counted: %v", got)
	}
	// Real user input at 9000 (far from any injection).
	if got := tr.idle(9500, 9000); got != 500*time.Millisecond {
		t.Fatalf("user input missed: %v", got)
	}
}

func TestIdleTrackerWraparound(t *testing.T) {
	var tr idleTracker
	last := uint32(0xFFFFFF00)
	now := last + 2000 // wraps
	if got := tr.idle(now, last); got != 2*time.Second {
		t.Fatalf("wrap: %v", got)
	}
	tr.injected(now - 50)
	if got := tr.idle(now+1000, now-40); got != 3*time.Second {
		t.Fatalf("own across wrap: %v", got)
	}
}

func TestIdleTrackerNeverGoesBackwards(t *testing.T) {
	var tr idleTracker
	tr.idle(5000, 4000)
	if got := tr.idle(6000, 3000); got != 2*time.Second {
		t.Fatalf("older tick replaced newer: %v", got)
	}
}

func TestUnicodeKeys(t *testing.T) {
	ins := unicodeKeys([]uint16{'1', ':'})
	if len(ins) != 4 {
		t.Fatalf("len %d", len(ins))
	}
	if ins[0].Ki.Scan != '1' || ins[0].Ki.Flags != keyeventfUnicode || ins[0].Type != inputKeyboard {
		t.Errorf("down: %+v", ins[0])
	}
	if ins[1].Ki.Flags != keyeventfUnicode|keyeventfKeyUp {
		t.Errorf("up: %+v", ins[1])
	}
}

func TestInputStructSizes(t *testing.T) {
	want := uintptr(28) // 32-bit INPUT
	if unsafe.Sizeof(uintptr(0)) == 8 {
		want = 40
	}
	if s := unsafe.Sizeof(mouseINPUT{}); s != want {
		t.Errorf("mouseINPUT size %d, want %d", s, want)
	}
	if s := unsafe.Sizeof(keybdINPUT{}); s != want {
		t.Errorf("keybdINPUT size %d, want %d", s, want)
	}
}

func TestCtrlChord(t *testing.T) {
	got := ctrlChord('S')
	want := []keybdINPUT{
		{Type: inputKeyboard, Ki: keybdInput{Vk: vkControl}},
		{Type: inputKeyboard, Ki: keybdInput{Vk: 'S'}},
		{Type: inputKeyboard, Ki: keybdInput{Vk: 'S', Flags: keyeventfKeyUp}},
		{Type: inputKeyboard, Ki: keybdInput{Vk: vkControl, Flags: keyeventfKeyUp}},
	}
	if len(got) != len(want) {
		t.Fatalf("len %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("input %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestQuitMethod(t *testing.T) {
	if quitMethod(true) != quitByWMClose {
		t.Error("classic Notepad (Edit child) must close via WM_CLOSE")
	}
	if quitMethod(false) != quitByCloseTab {
		t.Error("tabbed Notepad must close only our tab (Ctrl+W)")
	}
}

// fakeClock drives a quitPacer without real sleeping.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func (c *fakeClock) sleep(d time.Duration) {
	if d <= 0 {
		panic("non-positive sleep would spin")
	}
	c.t = c.t.Add(d)
}

func newTestPacer(budget time.Duration) (*fakeClock, quitPacer) {
	c := &fakeClock{t: time.Unix(1000, 0)}
	return c, quitPacer{deadline: c.t.Add(budget), now: c.now, sleep: c.sleep}
}

func TestQuitPacerSleepNeverPassesDeadline(t *testing.T) {
	c, p := newTestPacer(30 * time.Millisecond)
	n := 0
	for p.wait() {
		n++
	}
	if n != 2 { // 20 ms, then the remaining 10 ms
		t.Fatalf("waited %d polls, want 2", n)
	}
	if c.t.After(p.deadline) {
		t.Fatalf("slept past the deadline: %v > %v", c.t, p.deadline)
	}
}

func TestQuitPacerWithin(t *testing.T) {
	_, p := newTestPacer(3 * time.Second)
	q := p.within(500 * time.Millisecond)
	if got := q.deadline.Sub(p.now()); got != 500*time.Millisecond {
		t.Fatalf("sub-deadline %v, want 500ms", got)
	}
	_, short := newTestPacer(100 * time.Millisecond)
	if q := short.within(500 * time.Millisecond); !q.deadline.Equal(short.deadline) {
		t.Fatal("sub-deadline must never extend the overall deadline")
	}
}

func TestWaitQuietWaitsForQuietPeriodThenChecks(t *testing.T) {
	c, p := newTestPacer(3 * time.Second)
	start := c.t
	// User typed 100 ms ago; idle grows with the fake clock.
	idle := func() time.Duration { return 100*time.Millisecond + c.t.Sub(start) }
	checks := 0
	check := func() error { checks++; return nil }
	if err := waitQuiet(p, idle, check); err != nil {
		t.Fatal(err)
	}
	if idle() < quitQuiet {
		t.Fatalf("returned before the quiet period: idle %v", idle())
	}
	if checks == 0 {
		t.Fatal("ready check not run after the quiet period")
	}
}

func TestWaitQuietTimesOutWhileUserKeepsTyping(t *testing.T) {
	c, p := newTestPacer(time.Second)
	checks := 0
	err := waitQuiet(p, func() time.Duration { return 10 * time.Millisecond }, func() error { checks++; return nil })
	if err == nil {
		t.Fatal("must fail while the user keeps typing")
	}
	if c.t.After(p.deadline) {
		t.Fatal("waited past the deadline")
	}
}

func TestWaitQuietReturnsCheckError(t *testing.T) {
	_, p := newTestPacer(time.Second)
	want := errors.New("lost focus")
	if err := waitQuiet(p, func() time.Duration { return time.Hour }, func() error { return want }); !errors.Is(err, want) {
		t.Fatalf("got %v, want %v", err, want)
	}
}

// titleSeq returns successive window states, repeating the last one.
func titleSeq(states ...winState) func() winState {
	i := 0
	return func() winState {
		s := states[min(i, len(states)-1)]
		i++
		return s
	}
}

const testPath = `C:\Temp\wakebit-time.txt`

func TestWaitSaved(t *testing.T) {
	ours, dirty := "wakebit-time.txt - Notepad", "*wakebit-time.txt - Notepad"
	cases := []struct {
		name     string
		states   []winState
		wantErr  bool
		wantGone bool
	}{
		{"never dirty", []winState{{true, ours}}, false, false},
		{"clears after a few polls", []winState{{true, dirty}, {true, dirty}, {true, ours}}, false, false},
		{"never clears (read-only / Save As)", []winState{{true, dirty}}, true, false},
		{"title switches to another file", []winState{{true, dirty}, {true, "notes.txt - Notepad"}}, true, false},
		{"window disappears", []winState{{true, dirty}, {false, ""}}, true, true},
	}
	for _, c := range cases {
		clk, p := newTestPacer(time.Second)
		err := waitSaved(p, titleSeq(c.states...), testPath)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err = %v, wantErr %v", c.name, err, c.wantErr)
		}
		if errors.Is(err, errQuitWindowGone) != c.wantGone {
			t.Errorf("%s: gone = %v, want %v", c.name, errors.Is(err, errQuitWindowGone), c.wantGone)
		}
		if clk.t.After(p.deadline) {
			t.Errorf("%s: waited past the deadline", c.name)
		}
	}
}

func TestQuitDone(t *testing.T) {
	ours := "wakebit-time.txt - Notepad"
	cases := []struct {
		name      string
		st        winState
		elsewhere bool
		want      bool
	}{
		{"gone", winState{false, ""}, false, true},
		{"gone, file elsewhere", winState{false, ""}, true, true},
		{"still ours", winState{true, ours}, false, false},
		{"still ours dirty", winState{true, "*" + ours}, false, false},
		{"other title, file nowhere", winState{true, "notes.txt - Notepad"}, false, true},
		{"other title, file still shown elsewhere", winState{true, "notes.txt - Notepad"}, true, false},
	}
	for _, c := range cases {
		if got := quitDone(c.st, testPath, func() bool { return c.elsewhere }); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestWaitClosed(t *testing.T) {
	ours := "wakebit-time.txt - Notepad"
	_, p := newTestPacer(time.Second)
	if err := waitClosed(p, titleSeq(winState{true, ours}, winState{false, ""}), testPath, func() bool { return false }); err != nil {
		t.Fatalf("window closed: %v", err)
	}
	clk, p := newTestPacer(time.Second)
	if err := waitClosed(p, titleSeq(winState{true, ours}), testPath, func() bool { return false }); err == nil {
		t.Fatal("prompt left open must be an error")
	} else if clk.t.After(p.deadline) {
		t.Fatal("waited past the deadline")
	}
	_, p = newTestPacer(time.Second)
	err := waitClosed(p, titleSeq(winState{true, "notes.txt - Notepad"}), testPath, func() bool { return true })
	if err == nil || !strings.Contains(err.Error(), "switched or closed") {
		t.Fatalf("tab switched while our file is still shown: got %v", err)
	}
}

func TestModifierHeld(t *testing.T) {
	if modifierHeld([]uint16{0, 0x0001, 0}) {
		t.Error("toggle/low bits only must not count as held")
	}
	if !modifierHeld([]uint16{0, 0x8000}) {
		t.Error("high bit means held")
	}
	if !modifierHeld([]uint16{0x8001}) {
		t.Error("high bit with low bit means held")
	}
}
