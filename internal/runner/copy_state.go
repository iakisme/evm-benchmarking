// Package runner orchestrates double-pass replay execution.
package runner

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

// CopyState copies src to dst, preferring reflink (CoW) when supported.
// Falls back to a recursive byte copy with a warning to stderr.
func CopyState(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := tryReflinkCopy(src, dst); err == nil {
		return nil
	} else {
		fmt.Fprintf(os.Stderr, "evmbench: reflink copy failed (%v), falling back to full copy\n", err)
	}
	return walkCopy(src, dst)
}

// tryReflinkCopy uses `cp --reflink=auto -a src/. dst/`. If `cp` doesn't accept
// `--reflink` (BSD cp on macOS) or the FS doesn't support reflink, returns error.
func tryReflinkCopy(src, dst string) error {
	cp, err := exec.LookPath("cp")
	if err != nil {
		return err
	}
	args := []string{"--reflink=auto", "-a", src + string(filepath.Separator) + ".", dst}
	cmd := exec.Command(cp, args...)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func walkCopy(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(target, info.Mode())
		}
		if d.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// RemoveWorkdir cleans up a copied workdir.
func RemoveWorkdir(dst string) error {
	if err := os.RemoveAll(dst); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
