package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyDirCopiesAllFiles(t *testing.T) {
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "b"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "out")
	if err := CopyState(src, dst); err != nil {
		t.Fatalf("copy: %v", err)
	}

	a, err := os.ReadFile(filepath.Join(dst, "a"))
	if err != nil || string(a) != "hello" {
		t.Errorf("a = %q err=%v", a, err)
	}
	b, err := os.ReadFile(filepath.Join(dst, "sub", "b"))
	if err != nil || string(b) != "world" {
		t.Errorf("b = %q err=%v", b, err)
	}
}
