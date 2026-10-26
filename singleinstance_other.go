//go:build !windows

package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"syscall"

	"wakebit/internal/config"
)

// lockFileName lives next to config.json; it is never deleted.
const lockFileName = "wakebit.lock"

// acquireSingleInstance takes an exclusive flock on wakebit.lock in the
// config directory; ok is false when another instance already holds it.
// Other errors fail open (logged), like the Windows mutex.
func acquireSingleInstance() (release func(), ok bool) {
	path := filepath.Join(filepath.Dir(config.DefaultPath()), lockFileName)
	release, ok, err := acquireLock(path)
	if err != nil {
		log.Printf("wakebit: single-instance lock %s: %v (continuing without it)", path, err)
	}
	return release, ok
}

// acquireLock opens (creating if needed) path and takes a non-blocking
// exclusive flock on it. The file stays open until release, which unlocks
// and closes it but leaves the file in place. ok=false with a nil error
// means the lock is held elsewhere; a non-nil error means the lock could
// not be set up and the caller should fail open (ok is then true).
func acquireLock(path string) (release func(), ok bool, err error) {
	noop := func() {}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return noop, true, err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return noop, true, err
	}
	fd := int(f.Fd())
	if err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return noop, false, nil
		}
		return noop, true, fmt.Errorf("flock: %w", err)
	}
	return func() {
		_ = syscall.Flock(fd, syscall.LOCK_UN)
		f.Close()
	}, true, nil
}
