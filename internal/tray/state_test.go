package tray

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"wakebit/internal/config"
)

type applied struct {
	mode     config.Mode
	working  bool
	interval time.Duration
}

func newTestState(t *testing.T, c config.Config) (*state, *[]applied, string) {
	path := filepath.Join(t.TempDir(), "c.json")
	var calls []applied
	s := newState(path, c, func(m config.Mode, w bool, d time.Duration) {
		calls = append(calls, applied{m, w, d})
	})
	return s, &calls, path
}

func TestToggleWorkingSavesAndApplies(t *testing.T) {
	s, calls, path := newTestState(t, config.Config{Mode: config.ModeFull, Working: false, IntervalSec: 30})
	s.toggleWorking()
	if !s.cfg.Working {
		t.Fatal("expected working")
	}
	if got := config.Load(path); !got.Working || got.Mode != config.ModeFull {
		t.Fatalf("not saved: %+v", got)
	}
	want := applied{config.ModeFull, true, 30 * time.Second}
	if len(*calls) != 1 || (*calls)[0] != want {
		t.Fatalf("calls = %+v", *calls)
	}
	s.toggleWorking()
	if s.cfg.Working || (*calls)[1].working {
		t.Fatal("expected resting")
	}
}

func TestSetModeSavesAndApplies(t *testing.T) {
	s, calls, path := newTestState(t, config.Config{Mode: config.ModeLight, Working: true, IntervalSec: 60})
	s.setMode(config.ModeFull)
	if config.Load(path).Mode != config.ModeFull {
		t.Fatal("mode not saved")
	}
	if len(*calls) != 1 || (*calls)[0].mode != config.ModeFull || !(*calls)[0].working {
		t.Fatalf("calls = %+v", *calls)
	}
}

func TestSaveErrorDoesNotPreventApply(t *testing.T) {
	s, calls, _ := newTestState(t, config.Default())
	s.path = t.TempDir() // a directory: write fails
	s.toggleWorking()
	if len(*calls) != 1 {
		t.Fatal("apply must still run")
	}
}

func TestLabels(t *testing.T) {
	cases := []struct {
		c       config.Config
		tip, pb string
	}{
		{config.Config{Mode: config.ModeFull, Working: true}, "Wakebit: работает (Полный)", "Пауза"},
		{config.Config{Mode: config.ModeLight, Working: true}, "Wakebit: работает (Лёгкий)", "Пауза"},
		{config.Config{Mode: config.ModeLight, Working: false}, "Wakebit: отдыхает", "Продолжить"},
	}
	for _, c := range cases {
		if got := tooltip(c.c); got != c.tip {
			t.Errorf("tooltip = %q want %q", got, c.tip)
		}
		if got := toggleLabel(c.c); got != c.pb {
			t.Errorf("toggleLabel = %q want %q", got, c.pb)
		}
	}
}

func TestToggleAutostart(t *testing.T) {
	var got []bool
	ok := Autostart{Set: func(on bool) error { got = append(got, on); return nil }}
	if !toggleAutostart(ok, false) || toggleAutostart(ok, true) {
		t.Fatal("expected flip")
	}
	if len(got) != 2 || !got[0] || got[1] {
		t.Fatalf("calls %v", got)
	}
	bad := Autostart{Set: func(bool) error { return errors.New("boom") }}
	if toggleAutostart(bad, false) != false || toggleAutostart(bad, true) != true {
		t.Fatal("expected revert on error")
	}
}
