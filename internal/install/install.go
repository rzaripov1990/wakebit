// Package install handles the user-level install and autostart.
package install

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ErrUnsupported is returned on platforms without install support.
var ErrUnsupported = errors.New("install: not supported on this OS")

// InstallDir returns %LOCALAPPDATA%\Wakebit. It only ever returns an
// absolute path; otherwise it returns an error.
func InstallDir() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if !filepath.IsAbs(base) {
		home, err := os.UserHomeDir()
		if err != nil || !filepath.IsAbs(home) {
			return "", errors.New("install: cannot determine an absolute local app data directory")
		}
		base = filepath.Join(home, "AppData", "Local")
	}
	return filepath.Join(base, "Wakebit"), nil
}

// CopySelf copies the running executable into dstDir (same file name),
// overwriting an existing copy. If the running exe already is the
// destination, it is returned unchanged.
func CopySelf(dstDir string) (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	dst := filepath.Join(dstDir, filepath.Base(self))
	if err := copyFile(self, dst); err != nil {
		return "", fmt.Errorf("copy to %s (is the installed copy still running? quit it from the tray and retry): %w", dst, err)
	}
	return dst, nil
}

// copyFile copies src to dst, creating the directory and overwriting dst.
// It is a no-op when both paths are the same file.
func copyFile(src, dst string) error {
	si, err := os.Stat(src)
	if err != nil {
		return err
	}
	if di, err := os.Stat(dst); err == nil && os.SameFile(si, di) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// RemoveDir removes everything in dir except the file keep (best effort),
// then tries to remove dir itself. It refuses (deleting nothing) unless dir
// is an absolute, non-root path whose base name is "Wakebit".
func RemoveDir(dir, keep string) error {
	if !filepath.IsAbs(dir) {
		return fmt.Errorf("install: refusing to remove non-absolute path %q", dir)
	}
	dir = filepath.Clean(dir)
	if filepath.Dir(dir) == dir {
		return fmt.Errorf("install: refusing to remove root %q", dir)
	}
	if !strings.EqualFold(filepath.Base(dir), "Wakebit") {
		return fmt.Errorf("install: refusing to remove %q: not a Wakebit directory", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if keep != "" && samePath(p, keep) {
			continue
		}
		_ = os.RemoveAll(p)
	}
	_ = os.Remove(dir)
	return nil
}

func samePath(a, b string) bool {
	ai, err1 := os.Stat(a)
	bi, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(ai, bi)
}
