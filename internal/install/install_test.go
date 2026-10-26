package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopySelfCopiesFileAndOverwrites(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.exe")
	dst := filepath.Join(dir, "sub", "dst.exe")
	if err := os.WriteFile(src, []byte("v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if b, _ := os.ReadFile(dst); string(b) != "v1" {
		t.Fatalf("got %q", b)
	}
	if err := os.WriteFile(src, []byte("version-two"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	if b, _ := os.ReadFile(dst); string(b) != "version-two" {
		t.Fatalf("got %q", b)
	}
}

func TestCopyFileSameFileIsNoop(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.exe")
	if err := os.WriteFile(p, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(p, p); err != nil {
		t.Fatalf("same file: %v", err)
	}
	if b, _ := os.ReadFile(p); string(b) != "x" {
		t.Fatalf("content lost: %q", b)
	}
}

func TestRemoveDirRefusals(t *testing.T) {
	tmp := t.TempDir()
	other := filepath.Join(tmp, "Other")
	os.MkdirAll(other, 0o755)
	f := filepath.Join(other, "f")
	os.WriteFile(f, []byte("x"), 0o644)
	for _, d := range []string{filepath.Join("AppData", "Local", "Wakebit"), other, string(filepath.Separator), ""} {
		if err := RemoveDir(d, ""); err == nil {
			t.Errorf("expected refusal for %q", d)
		}
	}
	if _, err := os.Stat(f); err != nil {
		t.Fatal("file in wrong-name dir was touched")
	}
}

func TestRemoveDirKeepsExeAndOutsideFiles(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "Wakebit")
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	keep := filepath.Join(dir, "wakebit.exe")
	os.WriteFile(keep, []byte("x"), 0o755)
	os.WriteFile(filepath.Join(dir, "cfg.json"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "sub", "a"), []byte("x"), 0o644)
	sibling := filepath.Join(tmp, "sibling.txt")
	os.WriteFile(sibling, []byte("x"), 0o644)
	if err := RemoveDir(dir, keep); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Error("keep file removed")
	}
	if _, err := os.Stat(sibling); err != nil {
		t.Error("sibling removed")
	}
	for _, p := range []string{"cfg.json", "sub"} {
		if _, err := os.Stat(filepath.Join(dir, p)); err == nil {
			t.Errorf("%s not removed", p)
		}
	}
}

func TestInstallDirAbsolute(t *testing.T) {
	t.Setenv("LOCALAPPDATA", filepath.Join("rel", "x"))
	if d, err := localAppDataInstallDir(); err == nil && !filepath.IsAbs(d) {
		t.Fatalf("relative result %q", d)
	}
	base := t.TempDir()
	t.Setenv("LOCALAPPDATA", base)
	if d, err := localAppDataInstallDir(); err != nil || d != filepath.Join(base, "Wakebit") {
		t.Fatalf("got %q %v", d, err)
	}
}
