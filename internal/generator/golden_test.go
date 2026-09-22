package generator_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// update rewrites the golden fixtures instead of comparing against them.
var update = flag.Bool("update", false, "rewrite the golden fixtures")

// compareGolden asserts that got matches the fixture at path.
//
// Regenerate the fixtures with `go test ./internal/generator -update` whenever
// a template change is intentional.
func compareGolden(t *testing.T, path string, got []byte) {
	t.Helper()

	if *update {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, got, 0o644))
		return
	}

	want, err := os.ReadFile(path)
	require.NoError(t, err, "missing fixture %s: run `go test ./internal/generator -update`", path)
	require.Equal(t, string(want), string(got),
		"fixture %s is out of date: run `go test ./internal/generator -update`", path)
}

// goldenPath joins a fixture group and a file name under testdata.
func goldenPath(group, name string) string {
	return filepath.Join("testdata", group, name)
}
