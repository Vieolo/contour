package scaffold

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Move relocates the directory tree at src to dst.
//
// dst must not exist. Deciding what to do about an occupied destination belongs
// to the caller, which knows whether overwriting would lose someone's content.
//
// A rename handles the common case in one atomic step. It fails when src and dst
// sit on different filesystems — moving a store onto an external drive, say — so
// the tree is copied and the original removed only once the copy succeeds. A
// failed copy therefore leaves the source untouched.
func Move(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(dst), err)
	}

	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	if err := CopyTree(src, dst); err != nil {
		os.RemoveAll(dst) // never leave a half-written store behind
		return fmt.Errorf("copy %s to %s: %w", src, dst, err)
	}
	if err := os.RemoveAll(src); err != nil {
		return fmt.Errorf("remove %s after copying it to %s: %w", src, dst, err)
	}
	return nil
}

// CopyTree recursively copies the directory tree rooted at src into dst,
// creating dst and any parents. Non-regular files (symlinks, sockets) are
// skipped.
func CopyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
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
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if !d.Type().IsRegular() {
			return nil
		}
		return copyFile(path, target)
	})
}

// copyFile copies a single regular file, preserving its permission bits and
// creating the destination's parent directories as needed.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", src, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(dst), err)
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("copy %s to %s: %w", src, dst, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close %s: %w", dst, err)
	}
	return nil
}
