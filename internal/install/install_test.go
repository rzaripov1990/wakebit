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
