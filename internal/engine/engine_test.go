package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"testing"
	"time"

	"wakebit/internal/config"
)

type fakePlatform struct {
	mu         sync.Mutex
	sleepCalls []bool
	pos        [2]int
	moves      [][2]int
	idle       time.Duration
	editors    []*fakeEditor
	opens      int
	focusErr   error
	typeBlock  bool
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
	return f.idle
}
func (f *fakePlatform) OpenEditor(path string) (Editor, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opens++
	e := &fakeEditor{p: f, path: path}
	f.editors = append(f.editors, e)
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
func (e *fakeEditor) Close() {}

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
