//go:build !windows

package platform

import (
	"errors"
	"fmt"
	"math"
	"time"

	"wakebit/internal/engine"
)

// ErrUnsupported is returned by every operation of the stub platform.
var ErrUnsupported = errors.New("unsupported platform")

// unsupported is the engine.Platform for OSes without an implementation.
type unsupported struct{ goos string }

func (u unsupported) err() error { return fmt.Errorf("%w: %s", ErrUnsupported, u.goos) }

func (u unsupported) PreventSleep(bool) error                  { return u.err() }
func (u unsupported) CursorPos() (int, int, error)             { return 0, 0, u.err() }
func (u unsupported) MoveCursor(int, int) error                { return u.err() }
func (u unsupported) IdleFor() time.Duration                   { return time.Duration(math.MaxInt64) }
func (u unsupported) OpenEditor(string) (engine.Editor, error) { return nil, u.err() }
