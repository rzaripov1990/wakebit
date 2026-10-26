//go:build !windows

package platform

import (
	"errors"
	"testing"
	"time"
)

func TestUnsupportedPlatform(t *testing.T) {
	p := New()
	if err := p.PreventSleep(true); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("PreventSleep: got %v, want ErrUnsupported", err)
	}
	if _, _, err := p.CursorPos(); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("CursorPos: got %v", err)
	}
	if err := p.MoveCursor(1, 1); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("MoveCursor: got %v", err)
	}
	if ed, err := p.OpenEditor("x.txt"); ed != nil || !errors.Is(err, ErrUnsupported) {
		t.Fatalf("OpenEditor: got %v, %v", ed, err)
	}
	if d := p.IdleFor(); d < 24*time.Hour {
		t.Fatalf("IdleFor: got %v, want large", d)
	}
}
