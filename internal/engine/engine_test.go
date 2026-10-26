package engine

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"wakebit/internal/config"
)

type fakePlatform struct {
	mu          sync.Mutex
	sleepCalls  []bool
	pos         [2]int
	moves       [][2]int
	idle        time.Duration
	editors     []*fakeEditor
	opens       int
	focusErr    error
	typeBlock   bool
	openErr     error
	openGate    chan struct{} // if non-nil, OpenEditor signals openEntered and waits on it
	openEntered chan struct{}
	idleCalls   int
	focusCalls  int
	// Simulated user mouse move: once len(moves) reaches userMoveAfter,
	// CursorPos reports userPos (as if the user moved the mouse).
	userMoveAfter int
	userPos       [2]int
	userMoved     bool
	// Quitter support: when quitter is set, OpenEditor returns editors that
	// implement Quitter. events records "quit"/"close" calls in order.
	quitter bool
	quitErr error
	events  []string
}

func newFake() *fakePlatform { return &fakePlatform{idle: time.Hour, pos: [2]int{100, 200}} }

func (f *fakePlatform) PreventSleep(on bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sleepCalls = append(f.sleepCalls, on)
	return nil
}
func (f *fakePlatform) CursorPos() (int, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.userMoveAfter > 0 && !f.userMoved && len(f.moves) >= f.userMoveAfter {
		f.pos, f.userMoved = f.userPos, true
	}
	return f.pos[0], f.pos[1], nil
}
func (f *fakePlatform) MoveCursor(x, y int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pos = [2]int{x, y}
	f.moves = append(f.moves, [2]int{x, y})
	return nil
}
func (f *fakePlatform) IdleFor() time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.idleCalls++
	return f.idle
}
func (f *fakePlatform) OpenEditor(path string) (Editor, error) {
	f.mu.Lock()
	gate, entered := f.openGate, f.openEntered
	f.mu.Unlock()
	if gate != nil {
		entered <- struct{}{}
		<-gate // blocks like the real OpenEditor (no ctx)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opens++
	if f.openErr != nil {
		return nil, f.openErr
	}
	e := &fakeEditor{p: f, path: path}
	f.editors = append(f.editors, e)
	if f.quitter {
		return &quitFakeEditor{e}, nil
	}
	return e, nil
}
func (f *fakePlatform) snapshot() (sleep []bool, moves [][2]int, pos [2]int, typed []string, opens int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	sleep = append(sleep, f.sleepCalls...)
	moves = append(moves, f.moves...)
	for _, e := range f.editors {
		typed = append(typed, e.typed...)
	}
	return sleep, moves, f.pos, typed, f.opens
}

type fakeEditor struct {
	p     *fakePlatform
	path  string
	typed []string
	focus int
}

func (e *fakeEditor) Focus() error {
	e.p.mu.Lock()
	defer e.p.mu.Unlock()
	e.focus++
	e.p.focusCalls++
	return e.p.focusErr
}
func (e *fakeEditor) Type(ctx context.Context, text string, delay func() time.Duration) error {
	e.p.mu.Lock()
	block := e.p.typeBlock
	e.p.mu.Unlock()
	if block {
		<-ctx.Done()
		return ctx.Err()
	}
	e.p.mu.Lock()
	defer e.p.mu.Unlock()
	e.typed = append(e.typed, text)
	return nil
}
func (e *fakeEditor) Close() {
	e.p.mu.Lock()
	defer e.p.mu.Unlock()
	e.p.events = append(e.p.events, "close")
}

// quitFakeEditor is a fakeEditor that also implements Quitter.
type quitFakeEditor struct{ *fakeEditor }

func (e *quitFakeEditor) Quit() error {
	e.p.mu.Lock()
	defer e.p.mu.Unlock()
	e.p.events = append(e.p.events, "quit")
	return e.p.quitErr
}

func (f *fakePlatform) eventLog() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.events...)
}

func newTestEngine(p Platform, dir string) *Engine {
	e := New(p, dir)
	e.nudgeMin, e.nudgeMax, e.nudgeStep = 20*time.Millisecond, 30*time.Millisecond, 5*time.Millisecond
	return e
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("condition not met in time")
}

const iv = 10 * time.Millisecond

func TestWorkingBlocksSleepAndRestingReleases(t *testing.T) {
	f := newFake()
	e := newTestEngine(f, t.TempDir())
	e.Apply(config.ModeLight, true, iv)
	s, _, _, _, _ := f.snapshot()
	if len(s) != 1 || !s[0] {
		t.Fatalf("want [true], got %v", s)
	}
	e.Apply(config.ModeLight, false, iv)
	s, _, _, _, _ = f.snapshot()
	if len(s) != 2 || s[1] {
		t.Fatalf("want [true false], got %v", s)
	}
}

func TestLightCycleMovesCursorAndRestoresOriginal(t *testing.T) {
	f := newFake()
	e := newTestEngine(f, t.TempDir())
	e.Apply(config.ModeLight, true, time.Hour)
	waitFor(t, func() bool {
		_, m, p, _, _ := f.snapshot()
		return len(m) > 1 && p == [2]int{100, 200} && m[len(m)-1] == [2]int{100, 200}
	})
	e.Stop()
	_, m, p, _, _ := f.snapshot()
	if p != [2]int{100, 200} {
		t.Fatalf("cursor not restored: %v", p)
	}
	moved := false
	for _, mv := range m {
		if mv != [2]int{100, 200} {
			moved = true
		}
	}
	if !moved {
		t.Fatal("cursor never moved away")
	}
}

func TestLightCancelMidNudgeRestoresCursor(t *testing.T) {
	f := newFake()
	e := New(f, t.TempDir())
	e.nudgeMin, e.nudgeMax, e.nudgeStep = 5*time.Second, 5*time.Second, 2*time.Millisecond
	e.Apply(config.ModeLight, true, time.Hour)
	waitFor(t, func() bool { _, m, _, _, _ := f.snapshot(); return len(m) >= 2 })
	start := time.Now()
	e.Stop()
	if time.Since(start) > time.Second {
		t.Fatal("stop did not abort nudge promptly")
	}
	_, _, p, _, _ := f.snapshot()
	if p != [2]int{100, 200} {
		t.Fatalf("cursor not restored: %v", p)
	}
}

func TestFullCycleWritesTimeFileAndTypesHHMMSS(t *testing.T) {
	f := newFake()
	dir := t.TempDir()
	e := newTestEngine(f, dir)
	e.Apply(config.ModeFull, true, time.Hour)
	waitFor(t, func() bool { _, _, _, ty, _ := f.snapshot(); return len(ty) >= 1 })
	e.Stop()
	_, _, _, ty, _ := f.snapshot()
	if !regexp.MustCompile(`^\d{2}:\d{2}:\d{2}$`).MatchString(ty[0]) {
		t.Fatalf("bad typed text %q", ty[0])
	}
	f.mu.Lock()
	path := f.editors[0].path
	f.mu.Unlock()
	if filepath.Dir(path) != dir {
		t.Fatalf("file not in tempDir: %s", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestFullAbortsTypingOnStop(t *testing.T) {
	f := newFake()
	f.typeBlock = true
	e := newTestEngine(f, t.TempDir())
	e.Apply(config.ModeFull, true, time.Hour)
	waitFor(t, func() bool { f.mu.Lock(); defer f.mu.Unlock(); return len(f.editors) > 0 && f.editors[0].focus > 0 })
	done := make(chan struct{})
	go func() { e.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop blocked on typing")
	}
}

func TestCycleSkippedWhenUserActive(t *testing.T) {
	f := newFake()
	f.idle = time.Second
	e := newTestEngine(f, t.TempDir())
	e.Apply(config.ModeLight, true, iv)
	time.Sleep(60 * time.Millisecond)
	e.Apply(config.ModeFull, true, iv)
	time.Sleep(60 * time.Millisecond)
	e.Stop()
	s, m, _, ty, opens := f.snapshot()
	if len(m) != 0 || len(ty) != 0 || opens != 0 {
		t.Fatalf("simulated while user active: moves=%v typed=%v opens=%d", m, ty, opens)
	}
	if len(s) == 0 || !s[0] {
		t.Fatalf("sleep not blocked: %v", s)
	}
}

func TestEditorFocusErrorDoesNotCrashAndRetriesNextTick(t *testing.T) {
	f := newFake()
	f.focusErr = errors.New("window gone")
	e := newTestEngine(f, t.TempDir())
	e.Apply(config.ModeFull, true, iv)
	waitFor(t, func() bool { _, _, _, _, o := f.snapshot(); return o >= 3 })
	_, _, _, ty, _ := f.snapshot()
	if len(ty) != 0 {
		t.Fatalf("typed despite focus failure: %v", ty)
	}
	f.mu.Lock()
	f.focusErr = nil
	f.mu.Unlock()
	waitFor(t, func() bool { _, _, _, ty, _ := f.snapshot(); return len(ty) >= 1 })
	e.Stop()
}

func TestApplyIsIdempotent(t *testing.T) {
	f := newFake()
	e := newTestEngine(f, t.TempDir())
	e.Apply(config.ModeLight, true, iv)
	e.Apply(config.ModeLight, true, iv)
	e.Stop()
	s, _, _, _, _ := f.snapshot()
	n := 0
	for _, v := range s {
		if v {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("PreventSleep(true) called %d times: %v", n, s)
	}
}

func TestModeSwitchWhileWorkingChangesBehaviour(t *testing.T) {
	f := newFake()
	e := newTestEngine(f, t.TempDir())
	e.Apply(config.ModeLight, true, 20*time.Millisecond)
	waitFor(t, func() bool { _, m, _, _, _ := f.snapshot(); return len(m) > 0 })
	e.Apply(config.ModeFull, true, 20*time.Millisecond)
	waitFor(t, func() bool { _, _, _, ty, _ := f.snapshot(); return len(ty) > 0 })
	e.Stop()
	s, _, _, _, _ := f.snapshot()
	if len(s) != 2 || !s[0] || s[1] {
		t.Fatalf("sleep calls should be [true false], got %v", s)
	}
}

func TestOpenEditorFailureBacksOff(t *testing.T) {
	f := newFake()
	f.openErr = errors.New("notepad window did not appear")
	e := newTestEngine(f, t.TempDir())
	e.Apply(config.ModeFull, true, 2*time.Millisecond)
	// Each loop iteration polls IdleFor once (a failed open returns before
	// any re-check), so idleCalls counts cycles.
	waitFor(t, func() bool { f.mu.Lock(); defer f.mu.Unlock(); return f.idleCalls >= 4*(openBackoffCycles+1) })
	e.Stop()
	f.mu.Lock()
	cycles, opens := f.idleCalls, f.opens
	f.mu.Unlock()
	maxOpens := cycles/(openBackoffCycles+1) + 1
	if opens < 1 || opens > maxOpens {
		t.Fatalf("OpenEditor called %d times over %d cycles, want 1..%d", opens, cycles, maxOpens)
	}
}

func TestOpenEditorRecoversAfterBackoff(t *testing.T) {
	f := newFake()
	f.openErr = errors.New("timeout")
	e := newTestEngine(f, t.TempDir())
	e.Apply(config.ModeFull, true, 2*time.Millisecond)
	waitFor(t, func() bool { _, _, _, _, o := f.snapshot(); return o >= 1 })
	f.mu.Lock()
	f.openErr = nil
	f.mu.Unlock()
	waitFor(t, func() bool { _, _, _, ty, _ := f.snapshot(); return len(ty) >= 1 })
	e.Stop()
}

func blockingOpenFake() *fakePlatform {
	f := newFake()
	f.openGate = make(chan struct{})
	f.openEntered = make(chan struct{}, 1)
	return f
}

func TestNoFocusWhenUserReturnsDuringOpenEditor(t *testing.T) {
	f := blockingOpenFake()
	e := newTestEngine(f, t.TempDir())
	e.Apply(config.ModeFull, true, time.Hour)
	<-f.openEntered
	f.mu.Lock()
	f.idle = 0 // user came back while Notepad was starting
	f.mu.Unlock()
	close(f.openGate)
	waitFor(t, func() bool { _, _, _, _, o := f.snapshot(); return o == 1 })
	time.Sleep(30 * time.Millisecond)
	e.Stop()
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.focusCalls != 0 {
		t.Fatalf("Focus called %d times after user returned", f.focusCalls)
	}
}

func TestNoFocusWhenStoppedDuringOpenEditor(t *testing.T) {
	f := blockingOpenFake()
	e := newTestEngine(f, t.TempDir())
	e.Apply(config.ModeFull, true, time.Hour)
	<-f.openEntered
	done := make(chan struct{})
	go func() { e.Stop(); close(done) }()
	time.Sleep(30 * time.Millisecond) // let Stop cancel the loop context
	close(f.openGate)
	<-done
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.focusCalls != 0 {
		t.Fatalf("Focus called %d times after Stop", f.focusCalls)
	}
}

func TestLightNudgeYieldsToUserMouse(t *testing.T) {
	f := newFake()
	f.userMoveAfter = 3
	f.userPos = [2]int{500, 600}
	e := New(f, t.TempDir())
	e.nudgeMin, e.nudgeMax, e.nudgeStep = 5*time.Second, 5*time.Second, 2*time.Millisecond
	e.Apply(config.ModeLight, true, time.Hour)
	waitFor(t, func() bool { f.mu.Lock(); defer f.mu.Unlock(); return f.userMoved })
	time.Sleep(30 * time.Millisecond)
	start := time.Now()
	e.Stop()
	if time.Since(start) > time.Second {
		t.Fatal("nudge kept running after user took over")
	}
	_, m, p, _, _ := f.snapshot()
	if p != f.userPos {
		t.Fatalf("cursor moved away from user position: %v", p)
	}
	if len(m) != 3 {
		t.Fatalf("engine kept moving the cursor after the user took over: %v", m)
	}
	for _, mv := range m {
		if mv == [2]int{100, 200} {
			t.Fatalf("engine restored original position over user input: %v", m)
		}
	}
}

func TestApplyNonPositiveIntervalDoesNotPanic(t *testing.T) {
	f := newFake()
	e := newTestEngine(f, t.TempDir())
	e.Apply(config.ModeLight, true, 0)
	waitFor(t, func() bool { _, m, _, _, _ := f.snapshot(); return len(m) > 0 })
	e.Apply(config.ModeLight, true, -time.Second)
	e.Stop()
}

func quitFake() *fakePlatform {
	f := newFake()
	f.quitter = true
	return f
}

func countEvent(ev []string, name string) int {
	n := 0
	for _, v := range ev {
		if v == name {
			n++
		}
	}
	return n
}

// assertQuitThenClose checks the log is exactly [quit close].
func assertQuitThenClose(t *testing.T, ev []string) {
	t.Helper()
	if len(ev) != 2 || ev[0] != "quit" || ev[1] != "close" {
		t.Fatalf("want [quit close], got %v", ev)
	}
}

func startFullAndWaitTyped(t *testing.T, f *fakePlatform) *Engine {
	t.Helper()
	e := newTestEngine(f, t.TempDir())
	e.Apply(config.ModeFull, true, time.Hour)
	waitFor(t, func() bool { _, _, _, ty, _ := f.snapshot(); return len(ty) >= 1 })
	return e
}

func TestQuitCalledOnceBeforeCloseOnStop(t *testing.T) {
	f := quitFake()
	e := startFullAndWaitTyped(t, f)
	start := time.Now()
	e.Stop()
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Fatalf("Stop took %v with an immediate Quit", d)
	}
	assertQuitThenClose(t, f.eventLog())
	e.Stop() // already stopped: no second Quit
	if n := countEvent(f.eventLog(), "quit"); n != 1 {
		t.Fatalf("Quit called %d times", n)
	}
}

func TestQuitCalledOnSwitchToResting(t *testing.T) {
	f := quitFake()
	e := startFullAndWaitTyped(t, f)
	e.Apply(config.ModeFull, false, time.Hour)
	assertQuitThenClose(t, f.eventLog())
}

func TestQuitCalledOnModeChangeFullToLight(t *testing.T) {
	f := quitFake()
	e := startFullAndWaitTyped(t, f)
	e.Apply(config.ModeLight, true, time.Hour)
	assertQuitThenClose(t, f.eventLog())
	e.Stop()
	if n := countEvent(f.eventLog(), "quit"); n != 1 {
		t.Fatalf("Quit called %d times", n)
	}
}

func TestNoQuitForEditorWithoutQuitter(t *testing.T) {
	f := newFake()
	e := startFullAndWaitTyped(t, f)
	e.Stop()
	ev := f.eventLog()
	if countEvent(ev, "quit") != 0 || countEvent(ev, "close") != 1 {
		t.Fatalf("want [close], got %v", ev)
	}
}

func TestQuitErrorDoesNotBlockStopOrSleepRelease(t *testing.T) {
	f := quitFake()
	f.quitErr = errors.New("save dialog still open")
	var buf syncBuffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	e := startFullAndWaitTyped(t, f)
	done := make(chan struct{})
	go func() { e.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop hung after Quit error")
	}
	assertQuitThenClose(t, f.eventLog())
	s, _, _, _, _ := f.snapshot()
	if len(s) != 2 || !s[0] || s[1] {
		t.Fatalf("sleep calls should be [true false], got %v", s)
	}
	if !strings.Contains(buf.String(), "save dialog still open") {
		t.Fatalf("Quit error not logged; log: %q", buf.String())
	}
}

type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}
