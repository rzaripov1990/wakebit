//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAcquireLockExclusive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "wakebit.lock")

	release, ok, err := acquireLock(path)
	if err != nil || !ok {
		t.Fatalf("first acquire: ok=%v err=%v", ok, err)
	}
	if fi, err := os.Stat(path); err != nil {
		t.Fatalf("lock file: %v", err)
	} else if fi.Mode().Perm() != 0o600 {
		t.Errorf("lock file mode %v, want 0600", fi.Mode().Perm())
	}

	// flock is per open file description, so a second open in the same
	// process conflicts just like another process would.
	r2, ok, err := acquireLock(path)
	if err != nil || ok {
		t.Fatalf("second acquire while held: ok=%v err=%v, want ok=false err=nil", ok, err)
	}
	r2() // must be safe to call

	release()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("lock file must not be deleted on release: %v", err)
	}

	r3, ok, err := acquireLock(path)
	if err != nil || !ok {
		t.Fatalf("acquire after release: ok=%v err=%v", ok, err)
	}
	r3()
}

func TestAcquireLockUnwritableFailsOpen(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	release, ok, err := acquireLock(filepath.Join(dir, "sub", "wakebit.lock"))
	if err == nil || !ok {
		t.Fatalf("unwritable dir: ok=%v err=%v, want ok=true with an error", ok, err)
	}
	release()

	release, ok, err = acquireLock(filepath.Join(dir, "wakebit.lock"))
	if err == nil || !ok {
		t.Fatalf("unwritable file: ok=%v err=%v, want ok=true with an error", ok, err)
	}
	release()
}
