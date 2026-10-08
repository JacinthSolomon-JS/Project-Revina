package hardening

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// dropIn writes declarative configuration into a drop-in directory instead of
// editing a main config file. It refuses to rewrite a file it did not create.
type dropIn struct {
	path    string
	content string
	header  string // marker line that identifies our file
}

// write returns the original file bytes (for rollback) and whether it changed
// anything. It never touches files it did not create, and it backs up our own
// file before rewriting it. Targets that are not regular files (symlinks,
// devices) are refused: a root-run rule must never write through a symlink.
func (d dropIn) write() (original []byte, changed bool, err error) {
	orig, err := os.ReadFile(d.path)
	if err != nil && !os.IsNotExist(err) {
		return nil, false, fmt.Errorf("read %s: %w", d.path, err)
	}

	if err == nil {
		if string(orig) == d.content {
			return orig, false, nil
		}
		if !strings.Contains(string(orig), d.header) {
			return nil, false, fmt.Errorf("refusing to modify %s: file was not created by devsec", d.path)
		}
		backup := fmt.Sprintf("%s.devsec.bak.%d", d.path, time.Now().Unix())
		if err := os.WriteFile(backup, orig, 0o600); err != nil {
			return nil, false, fmt.Errorf("backup %s: %w", d.path, err)
		}
	} else if isSymlink(d.path) {
		return nil, false, fmt.Errorf("refusing to write %s: symlinks are not allowed", d.path)
	}

	if err := os.MkdirAll(filepath.Dir(d.path), 0o755); err != nil {
		return nil, false, fmt.Errorf("create dir for %s: %w", d.path, err)
	}
	if err := os.WriteFile(d.path, []byte(d.content), 0o644); err != nil {
		return nil, false, fmt.Errorf("write %s: %w", d.path, err)
	}
	if isSymlink(d.path) {
		// The path changed under us while writing; remove the artifact and fail.
		_ = os.Remove(d.path)
		return nil, false, fmt.Errorf("refusing to keep %s: became a symlink during write", d.path)
	}
	return orig, true, nil
}

// restore puts back previously-read original bytes (rollback on failed apply);
// an empty original means the file did not exist before Apply.
func (d dropIn) restore(original []byte) error {
	if isSymlink(d.path) {
		return fmt.Errorf("refusing to restore %s: path is a symlink", d.path)
	}
	if len(original) == 0 {
		return os.Remove(d.path)
	}
	return os.WriteFile(d.path, original, 0o644)
}

// isSymlink reports whether path exists and is a symbolic link.
func isSymlink(path string) bool {
	fi, err := os.Lstat(path)
	return err == nil && fi.Mode()&os.ModeSymlink != 0
}
