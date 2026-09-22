package generator_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
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
	require.Equal(t, stableEOL(string(want)), stableEOL(string(got)),
		"fixture %s is out of date: run `go test ./internal/generator -update`", path)
}

// stableEOL removes the carriage returns a Windows checkout may have added, so
// the comparison is about content and not about the platform that checked the
// repository out. `.gitattributes` marks the fixtures `-text` for the same
// reason; this keeps the suite honest when that rule is bypassed (a tarball
// download, a `git config core.autocrlf` override, an editor that rewrites EOL).
func stableEOL(s string) string {
	if !strings.Contains(s, "\r\n") {
		return s
	}
	return strings.ReplaceAll(s, "\r\n", "\n")
}

// goldenPath joins a fixture group and a file name under testdata.
func goldenPath(group, name string) string {
	return filepath.Join("testdata", group, name)
}
