package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileReturnsDefault(t *testing.T) {
	got := Load(filepath.Join(t.TempDir(), "nope.json"))
	if got != Default() {
		t.Fatalf("got %+v", got)
	}
}

func TestLoadCorruptJSONReturnsDefault(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.json")
	os.WriteFile(p, []byte("{not json"), 0o644)
	if got := Load(p); got != Default() {
		t.Fatalf("got %+v", got)
	}
}

func TestLoadInvalidModeOrIntervalFallsBack(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.json")
	os.WriteFile(p, []byte(`{"Mode":"x","Working":true,"IntervalSec":0}`), 0o644)
	got := Load(p)
	if got.Mode != ModeLight || got.IntervalSec != 60 || !got.Working {
		t.Fatalf("got %+v", got)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "c.json")
	want := Config{Mode: ModeFull, Working: true, IntervalSec: 30}
	if err := Save(p, want); err != nil {
		t.Fatal(err)
	}
	if got := Load(p); got != want {
		t.Fatalf("got %+v", got)
	}
}

func TestLoadClampsHugeInterval(t *testing.T) {
	cases := map[string]int{
		`{"Mode":"full","IntervalSec":86400}`:      86400,
		`{"Mode":"full","IntervalSec":86401}`:      86400,
		`{"Mode":"full","IntervalSec":2147483647}`: 86400,
		`{"Mode":"full","IntervalSec":-5}`:         60,
	}
	for in, want := range cases {
		p := filepath.Join(t.TempDir(), "c.json")
		os.WriteFile(p, []byte(in), 0o644)
		if got := Load(p).IntervalSec; got != want {
			t.Errorf("%s: IntervalSec = %d, want %d", in, got, want)
		}
	}
}
