// Package fsutil provides the small, dependency-free filesystem helpers that
// featherctl uses when writing generated files.
//
// Every helper here is deliberately conservative: nothing is ever overwritten
// unless the caller asks for it explicitly, so a generation run can never
// silently destroy work the user did by hand.
package fsutil

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ErrExists is the sentinel returned (wrapped in a [ConflictError]) whenever a
// write would overwrite an existing path without permission to do so.
var ErrExists = errors.New("path already exists")

// FileMode is the permission used for every generated file.
const FileMode fs.FileMode = 0o644

// DirMode is the permission used for every generated directory.
const DirMode fs.FileMode = 0o755

// ConflictError reports a path that already exists and was therefore not
// written.
type ConflictError struct {
	// Path is the offending path, as passed by the caller.
	Path string
	// IsDir reports whether the conflict is a directory.
	IsDir bool
	// Reason optionally explains the conflict in human terms.
	Reason string
}

// Error implements the error interface.
func (e *ConflictError) Error() string {
	if e.Reason != "" {
		return e.Reason
	}
	return fmt.Sprintf("%s already exists", e.Path)
}

// Unwrap allows callers to test conflicts with errors.Is(err, ErrExists).
func (e *ConflictError) Unwrap() error { return ErrExists }

// Exists reports whether path exists, regardless of its type.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsDir reports whether path exists and is a directory.
func IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// DirHasEntries reports whether path is a directory that contains at least one
// entry. A missing directory reports false, nil.
func DirHasEntries(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("read directory %s: %w", path, err)
	}
	return len(entries) > 0, nil
}

// EnsureDir creates path and every missing parent directory.
func EnsureDir(path string) error {
	if err := os.MkdirAll(path, DirMode); err != nil {
		return fmt.Errorf("create directory %s: %w", path, err)
	}
	return nil
}

// WriteFile creates path (and its parent directories) with data.
//
// When overwrite is false and path already exists, it returns a
// [*ConflictError] and leaves the existing file untouched.
func WriteFile(path string, data []byte, overwrite bool) error {
	if err := EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}

	if Exists(path) && !overwrite {
		info, err := os.Stat(path)
		isDir := err == nil && info.IsDir()
		return &ConflictError{Path: path, IsDir: isDir}
	}

	if err := os.WriteFile(path, data, FileMode); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// RemoveAll deletes path and everything below it. A missing path is not an
// error.
func RemoveAll(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

// Rel returns path expressed relative to root, using forward slashes so the
// result is stable across operating systems. Paths outside root are returned
// unchanged.
func Rel(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	rel = filepath.ToSlash(rel)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return filepath.ToSlash(path)
	}
	return rel
}
