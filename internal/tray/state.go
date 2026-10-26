package tray

import (
	"log"
	"time"

	"wakebit/internal/config"
)

// applyFunc pushes the desired state into the engine.
type applyFunc func(mode config.Mode, working bool, interval time.Duration)

// state holds the UI-independent logic: every change updates the config,
// persists it and applies it to the engine.
type state struct {
	path  string
	cfg   config.Config
	apply applyFunc
}

func newState(path string, c config.Config, apply applyFunc) *state {
	return &state{path: path, cfg: c, apply: apply}
}

func (s *state) commit() {
	if err := config.Save(s.path, s.cfg); err != nil {
		log.Printf("tray: save config: %v", err)
	}
	s.apply(s.cfg.Mode, s.cfg.Working, s.interval())
}

func (s *state) toggleWorking() {
	s.cfg.Working = !s.cfg.Working
	s.commit()
}

func (s *state) setMode(m config.Mode) {
	s.cfg.Mode = m
	s.commit()
}

func modeName(m config.Mode) string {
	if m == config.ModeFull {
		return "Полный"
	}
	return "Лёгкий"
}

func tooltip(c config.Config) string {
	if !c.Working {
		return "Wakebit: отдыхает"
	}
	return "Wakebit: работает (" + modeName(c.Mode) + ")"
}

func toggleLabel(c config.Config) string {
	if c.Working {
		return "Пауза"
	}
	return "Продолжить"
}

func (s *state) interval() time.Duration {
	return time.Duration(s.cfg.IntervalSec) * time.Second
}
