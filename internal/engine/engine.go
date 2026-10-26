// Package engine keeps the computer awake by blocking sleep and simulating activity.
package engine

import (
	"context"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"wakebit/internal/config"
)

const minIdle = 5 * time.Second

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
	var ed Editor
	var path string
	defer func() {
		if ed != nil {
			ed.Close()
		}
	}()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if e.p.IdleFor() >= minIdle {
			switch mode {
			case config.ModeFull:
				e.fullCycle(ctx, &ed, &path)
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

func (e *Engine) lightCycle(ctx context.Context) {
	x, y, err := e.p.CursorPos()
	if err != nil {
		log.Printf("engine: cursor pos: %v", err)
		return
	}
	defer func() {
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
		d := path[i%len(path)]
		if err := e.p.MoveCursor(x+d[0], y+d[1]); err != nil {
			log.Printf("engine: move cursor: %v", err)
			return
		}
		if !sleepCtx(ctx, e.nudgeStep) {
			return
		}
	}
}

func (e *Engine) fullCycle(ctx context.Context, ed *Editor, path *string) {
	if *ed == nil {
		p := filepath.Join(e.tempDir, "wakebit-time.txt")
		if err := os.WriteFile(p, []byte(time.Now().Format("15:04:05")+"\n"), 0o644); err != nil {
			log.Printf("engine: write time file: %v", err)
			return
		}
		opened, err := e.p.OpenEditor(p)
		if err != nil {
			log.Printf("engine: open editor: %v", err)
			return
		}
		*ed, *path = opened, p
	}
	if err := (*ed).Focus(); err != nil {
		log.Printf("engine: focus editor (will retry next tick): %v", err)
		(*ed).Close()
		*ed = nil
		return
	}
	text := time.Now().Format("15:04:05")
	if err := (*ed).Type(ctx, text, e.typeDelay); err != nil && ctx.Err() == nil {
		log.Printf("engine: type: %v", err)
		(*ed).Close()
		*ed = nil
	}
}
