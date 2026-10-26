package winplat

import (
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
		{"*wakebit-time.txt - Notepad", true},
		{"WAKEBIT-TIME.TXT - Блокнот", true},
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
		{hwnd: 6, title: "*wakebit-time.txt - Notepad", class: "Notepad"},
	}
	if got := pickWindow(cands, p); got != 6 {
		t.Fatalf("got %d, want 6", got)
	}
	// Same base name in another app, or a different file in Notepad: rejected.
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
