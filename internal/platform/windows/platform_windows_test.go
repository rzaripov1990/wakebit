//go:build windows

package winplat

import (
	"testing"
	"unsafe"

	"wakebit/internal/engine"
)

// These tests make no GUI or power-state changes.

func TestNewImplementsPlatform(t *testing.T) {
	var _ engine.Platform = New()
}

func TestWin32StructSizes(t *testing.T) {
	ptr := unsafe.Sizeof(uintptr(0))
	// REASON_CONTEXT: 2*ULONG + union of {HMODULE, ULONG, ULONG, LPWSTR*}.
	if want := 8 + 2*ptr + 8; unsafe.Sizeof(reasonContext{}) != want {
		t.Errorf("reasonContext size %d, want %d", unsafe.Sizeof(reasonContext{}), want)
	}
	if unsafe.Sizeof(lastInputInfo{}) != 8 {
		t.Errorf("lastInputInfo size %d", unsafe.Sizeof(lastInputInfo{}))
	}
	if unsafe.Sizeof(point{}) != 8 {
		t.Errorf("point size %d", unsafe.Sizeof(point{}))
	}
}

func TestIdleForIsReadable(t *testing.T) {
	// Reads GetLastInputInfo only; must not panic and must be non-negative.
	if d := (&Platform{}).IdleFor(); d < 0 {
		t.Fatalf("negative idle %v", d)
	}
}
