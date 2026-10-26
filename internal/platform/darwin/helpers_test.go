package darwinplat

import (
	"math"
	"strings"
	"testing"
	"time"
)

const testPath = "/var/folders/xy/T/wakebit-time.txt"

func TestWindowNameMatches(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"wakebit-time.txt", true},
		{"WAKEBIT-TIME.TXT", true},            // default APFS is case-insensitive
		{"wakebit-time", true},                // extension hidden
		{"wakebit-time.txt — Edited", true},   // em-dash status suffix
		{"wakebit-time.txt — Изменено", true}, // localized suffix
		{"wakebit-time — Edited", true},
		{"wakebit-time.txt - copy.txt", false}, // plain hyphen: another file
		{"old-wakebit-time.txt", false},
		{"wakebit-time.txt.bak", false},
		{"wakebit-time.txt — ", false},
		{"Untitled", false},
		{"", false},
	}
	for _, c := range cases {
		if got := windowNameMatches(c.name, testPath); got != c.want {
			t.Errorf("windowNameMatches(%q) = %v, want %v", c.name, got, c.want)
		}
	}
	for _, p := range []string{"", "/", "."} {
		if windowNameMatches(p, p) || windowNameMatches("x", p) {
			t.Errorf("degenerate path %q must not match", p)
		}
	}
}

func TestPickWindowIndex(t *testing.T) {
	names := []string{"Untitled", "old-wakebit-time.txt", "wakebit-time.txt — Edited", "wakebit-time.txt"}
	if got := pickWindowIndex(names, testPath); got != 3 {
		t.Fatalf("got %d, want 3 (1-based, first match)", got)
	}
	if got := pickWindowIndex(names[:2], testPath); got != 0 {
		t.Fatalf("got %d, want 0", got)
	}
	if got := pickWindowIndex(nil, testPath); got != 0 {
		t.Fatalf("nil: got %d", got)
	}
}

func TestSplitLines(t *testing.T) {
	got := splitLines("a\nb c\r\n\n")
	if len(got) != 2 || got[0] != "a" || got[1] != "b c" {
		t.Fatalf("got %q", got)
	}
	if got := splitLines(""); len(got) != 0 {
		t.Fatalf("empty: %q", got)
	}
	// A blank line in the middle keeps its position (window indices!).
	if got := splitLines("a\n\nb\n"); len(got) != 3 || got[1] != "" {
		t.Fatalf("middle blank: %q", got)
	}
}

func TestParseFront(t *testing.T) {
	b, n := parseFront("com.apple.TextEdit\nwakebit-time.txt\n")
	if b != textEditBundle || n != "wakebit-time.txt" {
		t.Fatalf("got %q %q", b, n)
	}
	b, n = parseFront("com.apple.Terminal\n")
	if b != "com.apple.Terminal" || n != "" {
		t.Fatalf("got %q %q", b, n)
	}
}

func TestFrontIsEditor(t *testing.T) {
	if !frontIsEditor("com.apple.TextEdit\nwakebit-time.txt\n", testPath) {
		t.Error("TextEdit with our file must be accepted")
	}
	for _, out := range []string{
		"com.apple.Terminal\nwakebit-time.txt\n",
		"com.apple.TextEdit\nother.txt\n",
		"com.apple.TextEdit\n",
		"",
	} {
		if frontIsEditor(out, testPath) {
			t.Errorf("accepted %q", out)
		}
	}
}

func TestFocusScript(t *testing.T) {
	s := focusScript(3)
	if !strings.Contains(s, "set index of window 3 to 1") || !strings.Contains(s, "activate") {
		t.Fatalf("script:\n%s", s)
	}
}

func TestScriptArgs(t *testing.T) {
	args := scriptArgs("line1\nline2")
	want := []string{"-e", "line1", "-e", "line2"}
	if strings.Join(args, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q", args)
	}
}

func TestRuneUnits(t *testing.T) {
	if a, _, n := runeUnits('7'); n != 1 || a != '7' {
		t.Errorf("ascii: %x %d", a, n)
	}
	if a, _, n := runeUnits('ж'); n != 1 || a != 0x0436 {
		t.Errorf("bmp: %x %d", a, n)
	}
	if a, b, n := runeUnits('😀'); n != 2 || a != 0xD83D || b != 0xDE00 {
		t.Errorf("astral: %x %x %d", a, b, n)
	}
	if a, _, n := runeUnits(0xD800); n != 1 || a != 0xFFFD {
		t.Errorf("invalid: %x %d", a, n)
	}
}

func TestSecondsToDuration(t *testing.T) {
	if got := secondsToDuration(1.5); got != 1500*time.Millisecond {
		t.Errorf("1.5s: %v", got)
	}
	for _, v := range []float64{-1, math.NaN()} {
		if got := secondsToDuration(v); got != 0 {
			t.Errorf("%v: %v", v, got)
		}
	}
	if got := secondsToDuration(math.Inf(1)); got < 24*time.Hour {
		t.Errorf("inf: %v", got)
	}
	if got := secondsToDuration(1e300); got < 24*time.Hour {
		t.Errorf("huge: %v", got)
	}
}

func TestIdleTrackerIgnoresOwnInput(t *testing.T) {
	var tr idleTracker
	// Last real input 6s before now=7000.
	if got := tr.idle(7000, 6*time.Second); got != 6*time.Second {
		t.Fatalf("initial: %v", got)
	}
	// We inject at 7000; at 8000 the system reports last input 990ms ago
	// (tick 7010, ours within tolerance) -> still 7s since the user.
	tr.injected(7000)
	if got := tr.idle(8000, 990*time.Millisecond); got != 7*time.Second {
		t.Fatalf("own input counted: %v", got)
	}
	// Real user input at 9000.
	if got := tr.idle(9500, 500*time.Millisecond); got != 500*time.Millisecond {
		t.Fatalf("user input missed: %v", got)
	}
}

func TestIdleTrackerNeverGoesBackwards(t *testing.T) {
	var tr idleTracker
	tr.idle(5000, time.Second) // user at 4000
	if got := tr.idle(6000, 3*time.Second); got != 2*time.Second {
		t.Fatalf("older tick replaced newer: %v", got)
	}
}

func TestIdleTrackerHugeIdle(t *testing.T) {
	var tr idleTracker
	// No input since long before the process started: must not go negative.
	if got := tr.idle(100, 1000*time.Hour); got < 999*time.Hour {
		t.Fatalf("got %v", got)
	}
}
