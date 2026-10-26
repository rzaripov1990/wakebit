//go:build darwin

package darwinplat

import (
	"testing"

	"wakebit/internal/engine"
)

// These run on the macOS CI job only. They make no GUI changes; the power
// assertions are released before returning.

func TestNewImplementsPlatform(t *testing.T) {
	var _ engine.Platform = New()
}

func TestPreventSleepIdempotent(t *testing.T) {
	p := New()
	for _, on := range []bool{true, true, false, false} {
		if err := p.PreventSleep(on); err != nil {
			t.Fatalf("PreventSleep(%v): %v", on, err)
		}
	}
}

func TestIdleForIsReadable(t *testing.T) {
	if d := New().IdleFor(); d < 0 {
		t.Fatalf("negative idle %v", d)
	}
}
