package fsutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cybersafetyid/featherctl/internal/fsutil"
)

func TestWriteFileCreatesParents(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "a", "b", "c.go")
	require.NoError(t, fsutil.WriteFile(path, []byte("package a\n"), false))

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "package a\n", string(content))
}

func TestWriteFileRefusesToOverwrite(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "main.go")
	require.NoError(t, fsutil.WriteFile(path, []byte("first"), false))

	err := fsutil.WriteFile(path, []byte("second"), false)
	require.Error(t, err)
	assert.ErrorIs(t, err, fsutil.ErrExists)

	var conflict *fsutil.ConflictError
	require.ErrorAs(t, err, &conflict)
	assert.Equal(t, path, conflict.Path)
	assert.False(t, conflict.IsDir)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "first", string(content), "the original file must survive a refused write")
}

func TestWriteFileOverwritesWhenAsked(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "main.go")
	require.NoError(t, fsutil.WriteFile(path, []byte("first"), false))
	require.NoError(t, fsutil.WriteFile(path, []byte("second"), true))

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "second", string(content))
}

func TestWriteFileRefusesToOverwriteADirectory(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "adir")
	require.NoError(t, os.MkdirAll(path, 0o755))

	err := fsutil.WriteFile(path, []byte("data"), false)
	require.Error(t, err)

	var conflict *fsutil.ConflictError
	require.ErrorAs(t, err, &conflict)
	assert.True(t, conflict.IsDir)
	assert.True(t, fsutil.IsDir(path), "the directory must survive")
}

func TestConflictErrorMessage(t *testing.T) {
	t.Parallel()

	plain := &fsutil.ConflictError{Path: "internal/features/order"}
	assert.Equal(t, "internal/features/order already exists", plain.Error())

	explained := &fsutil.ConflictError{Path: "x", Reason: "use --force"}
	assert.Equal(t, "use --force", explained.Error())
}

func TestExistsAndIsDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	file := filepath.Join(root, "file")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o644))

	assert.True(t, fsutil.Exists(file))
	assert.False(t, fsutil.IsDir(file))
	assert.True(t, fsutil.IsDir(root))
	assert.False(t, fsutil.Exists(filepath.Join(root, "nope")))
	assert.False(t, fsutil.IsDir(filepath.Join(root, "nope")))
}

func TestDirHasEntries(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	empty, err := fsutil.DirHasEntries(root)
	require.NoError(t, err)
	assert.False(t, empty)

	require.NoError(t, os.WriteFile(filepath.Join(root, "file"), []byte("x"), 0o644))

	nonEmpty, err := fsutil.DirHasEntries(root)
	require.NoError(t, err)
	assert.True(t, nonEmpty)

	missing, err := fsutil.DirHasEntries(filepath.Join(root, "nope"))
	require.NoError(t, err)
	assert.False(t, missing)
}

func TestEnsureDirIsIdempotent(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "deep", "nested")
	require.NoError(t, fsutil.EnsureDir(path))
	require.NoError(t, fsutil.EnsureDir(path))
	assert.True(t, fsutil.IsDir(path))
}

func TestRemoveAll(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gone")
	require.NoError(t, fsutil.WriteFile(filepath.Join(path, "file"), []byte("x"), false))
	require.NoError(t, fsutil.RemoveAll(path))
	assert.False(t, fsutil.Exists(path))
	require.NoError(t, fsutil.RemoveAll(path), "removing a missing path is not an error")
}

func TestRel(t *testing.T) {
	t.Parallel()

	root := filepath.Join("root")

	assert.Equal(t, "internal/features/order/model.go",
		fsutil.Rel(root, filepath.Join(root, "internal", "features", "order", "model.go")))
	assert.Equal(t, filepath.ToSlash(filepath.Join("..", "other")),
		fsutil.Rel(root, filepath.Join("..", "other")))
}
