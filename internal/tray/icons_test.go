package tray

import (
	"bytes"
	"runtime"
	"testing"
)

func TestIconsAreValidAndDistinct(t *testing.T) {
	on, off := icon(true), icon(false)
	if len(on) == 0 || len(off) == 0 {
		t.Fatal("empty icon")
	}
	if bytes.Equal(on, off) {
		t.Fatal("working and resting icons must differ")
	}
	for _, b := range [][]byte{on, off} {
		if runtime.GOOS == "windows" {
			if !bytes.HasPrefix(b, []byte{0, 0, 1, 0}) {
				t.Fatalf("bad ICO header: % x", b[:4])
			}
		} else if !bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")) {
			t.Fatalf("bad PNG signature: % x", b[:8])
		}
	}
}
