// Package engine keeps the computer awake by blocking sleep and simulating activity.
package engine

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"wakebit/internal/config"
)

const minIdle = 5 * time.Second

// openBackoffCycles is how many cycles Full mode skips after OpenEditor
// fails, so a Notepad that never shows its window is not respawned every
// interval.
const openBackoffCycles = 3

// defaultInterval replaces a non-positive interval (time.NewTicker panics
// on those).
const defaultInterval = 60 * time.Second

// userMoveTolerance is how far (px) the cursor may be from where the Light
// nudge last put it before we conclude the user is moving the mouse.
const userMoveTolerance = 2

// Engine runs the Working/Resting state machine.
type Engine struct {
	p       Platform
	tempDir string

	// Injectable for tests.
	nudgeMin, nudgeMax, nudgeStep time.Duration
	typeDelay                     func() time.Duration

	mu       sync.Mutex
	running  bool
	mode     config.Mode
	interval time.Duration
	cancel   context.CancelFunc
	done     chan struct{}
}

func New(p Platform, tempDir string) *Engine {
	return &Engine{
		p:         p,
		tempDir:   tempDir,
		nudgeMin:  time.Second,
		nudgeMax:  2 * time.Second,
		nudgeStep: 50 * time.Millisecond,
		typeDelay: func() time.Duration { return time.Duration(30+rand.Intn(90)) * time.Millisecond },
	}
}

// Apply is idempotent: it (re)starts or stops the loop only when state changes.
func (e *Engine) Apply(mode config.Mode, working bool, interval time.Duration) {
	if interval <= 0 {
		interval = defaultInterval
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if working && e.running && e.mode == mode && e.interval == interval {
		return
	}
	if !working && !e.running {
		return
	}
	wasRunning := e.running
	e.stopLoopLocked()
	if !working {
		if err := e.p.PreventSleep(false); err != nil {
			log.Printf("engine: release sleep: %v", err)
		}
		return
	}
	if !wasRunning {
		if err := e.p.PreventSleep(true); err != nil {
			log.Printf("engine: prevent sleep: %v", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	e.running, e.mode, e.interval, e.cancel, e.done = true, mode, interval, cancel, done
	go e.loop(ctx, done, mode, interval)
}

// Stop releases sleep blocking and blocks until the loop exits.
func (e *Engine) Stop() {
	e.mu.Lock()
	m, i := e.mode, e.interval
	e.mu.Unlock()
	e.Apply(m, false, i)
}

func (e *Engine) stopLoopLocked() {
	if !e.running {
		return
	}
	e.cancel()
	<-e.done
	e.running = false
}

func (e *Engine) loop(ctx context.Context, done chan struct{}, mode config.Mode, interval time.Duration) {
	defer close(done)
	var fs fullState
	defer fs.release()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if e.p.IdleFor() >= minIdle {
			switch mode {
			case config.ModeFull:
				e.fullCycle(ctx, &fs)
			default:
				e.lightCycle(ctx)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// sleepCtx waits d or until ctx is cancelled; reports whether ctx is still live.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// lightCycle nudges the cursor a few pixels back and forth and then puts it
// back. If the cursor is found somewhere the engine did not put it, the user
// has taken over: the nudge stops and the original position is NOT restored.
func (e *Engine) lightCycle(ctx context.Context) {
	x, y, err := e.p.CursorPos()
	if err != nil {
		log.Printf("engine: cursor pos: %v", err)
		return
	}
	lx, ly := x, y // where the engine last put the cursor
	userMoved := func() bool {
		cx, cy, err := e.p.CursorPos()
		if err != nil {
			return false
		}
		return abs(cx-lx) > userMoveTolerance || abs(cy-ly) > userMoveTolerance
	}
	defer func() {
		if userMoved() {
			return
		}
		if err := e.p.MoveCursor(x, y); err != nil {
			log.Printf("engine: restore cursor: %v", err)
		}
	}()
	dur := e.nudgeMin
	if e.nudgeMax > e.nudgeMin {
		dur += time.Duration(rand.Int63n(int64(e.nudgeMax - e.nudgeMin)))
	}
	// Small back-and-forth path of a few pixels.
	path := [][2]int{{1, 0}, {2, 1}, {3, 2}, {2, 1}, {1, 0}}
	deadline := time.Now().Add(dur)
	for i := 0; ctx.Err() == nil && time.Now().Before(deadline); i++ {
		if userMoved() {
			return // deferred restore sees the move too and skips
		}
		d := path[i%len(path)]
		if err := e.p.MoveCursor(x+d[0], y+d[1]); err != nil {
			log.Printf("engine: move cursor: %v", err)
			return
		}
		lx, ly = x+d[0], y+d[1]
		if !sleepCtx(ctx, e.nudgeStep) {
			return
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// fullState is Full mode's per-loop state.
type fullState struct {
	ed   Editor
	path string
	skip int // cycles left to skip after an OpenEditor failure
}

// release closes the editor window (when the editor supports Quitter) and
// then the handle. It runs when the loop ends; Stop waits for it.
func (fs *fullState) release() {
	if fs.ed == nil {
		return
	}
	if q, ok := fs.ed.(Quitter); ok {
		if err := q.Quit(); err != nil {
			log.Printf("engine: close editor window: %v", err)
		}
	}
	fs.ed.Close()
	fs.ed = nil
}

func (e *Engine) fullCycle(ctx context.Context, fs *fullState) {
	if fs.ed == nil {
		if fs.skip > 0 {
			fs.skip--
			return
		}
		p := filepath.Join(e.tempDir, "wakebit-time.txt")
		if err := os.WriteFile(p, []byte(time.Now().Format("15:04:05")+"\n"), 0o644); err != nil {
			log.Printf("engine: write time file: %v", err)
			return
		}
		// OpenEditor may block for seconds and takes no ctx.
		opened, err := e.p.OpenEditor(p)
		if err != nil {
			log.Printf("engine: open editor (retrying after %d cycles): %v", openBackoffCycles, err)
			fs.skip = openBackoffCycles
			return
		}
		fs.ed, fs.path = opened, p
		// The world may have changed while we waited: never grab focus after
		// Stop or once the user is back.
		if ctx.Err() != nil || e.p.IdleFor() < minIdle {
			return
		}
	}
	if err := fs.ed.Focus(); err != nil {
		log.Printf("engine: focus editor (will retry next tick): %v", err)
		fs.dropOnError(err)
		return
	}
	text := time.Now().Format("15:04:05")
	if err := fs.ed.Type(ctx, text, e.typeDelay); err != nil && ctx.Err() == nil {
		log.Printf("engine: type: %v", err)
		fs.dropOnError(err)
	}
}

// dropOnError decides what happens to the editor handle after a Focus or
// Type error. A window that is definitely gone (ErrEditorGone) is dropped
// without Quit. Otherwise an editor that implements Quitter is kept, so the
// next cycle re-focuses it and Quit can still close it when Full mode ends;
// other editors are dropped as before (reopened next cycle).
func (fs *fullState) dropOnError(err error) {
	if !errors.Is(err, ErrEditorGone) {
		if _, ok := fs.ed.(Quitter); ok {
			return
		}
	}
	fs.ed.Close()
	fs.ed = nil
}
