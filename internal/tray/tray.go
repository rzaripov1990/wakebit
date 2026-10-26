// Package tray wires the system tray UI to the engine.
package tray

import (
	"sync"

	"fyne.io/systray"

	"wakebit/internal/config"
	"wakebit/internal/engine"
)

// Run shows the tray icon and blocks until the user quits.
func Run(cfgPath string, eng *engine.Engine) {
	s := newState(cfgPath, config.Load(cfgPath), eng.Apply)
	systray.Run(func() { onReady(s) }, func() {})
}

func onReady(s *state) {
	mFull := systray.AddMenuItemCheckbox("Режим: Полный", "", false)
	mLight := systray.AddMenuItemCheckbox("Режим: Лёгкий", "", false)
	mToggle := systray.AddMenuItem("", "")
	systray.AddSeparator()
	// TODO(Task 5): wire autostart (registry Run key); disabled placeholder until then.
	mAuto := systray.AddMenuItemCheckbox("Автозапуск", "", false)
	mAuto.Disable()
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Выход", "")

	var mu sync.Mutex // tap handler and menu loop run on different goroutines
	refresh := func() {
		systray.SetIcon(icon(s.cfg.Working))
		systray.SetTooltip(tooltip(s.cfg))
		systray.SetTitle(tooltip(s.cfg))
		mToggle.SetTitle(toggleLabel(s.cfg))
		setChecked(mFull, s.cfg.Mode == config.ModeFull)
		setChecked(mLight, s.cfg.Mode == config.ModeLight)
	}

	// Apply the persisted state at startup (without rewriting the config).
	s.apply(s.cfg.Mode, s.cfg.Working, s.interval())
	refresh()

	systray.SetOnTapped(func() {
		mu.Lock()
		defer mu.Unlock()
		s.toggleWorking()
		refresh()
	})

	go func() {
		for {
			select {
			case <-mFull.ClickedCh:
				mu.Lock()
				s.setMode(config.ModeFull)
			case <-mLight.ClickedCh:
				mu.Lock()
				s.setMode(config.ModeLight)
			case <-mToggle.ClickedCh:
				mu.Lock()
				s.toggleWorking()
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
			refresh()
			mu.Unlock()
		}
	}()
}

func setChecked(m *systray.MenuItem, v bool) {
	if v {
		m.Check()
	} else {
		m.Uncheck()
	}
}
