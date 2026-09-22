package generator_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cybersafetyid/featherctl/internal/generator"
)

// initProject scaffolds a project into a fresh temporary directory.
func initProject(t *testing.T, name, module string, opts ...func(*generator.ProjectOptions)) (*generator.Report, string) {
	t.Helper()

	dir := filepath.Join(t.TempDir(), name)
	options := generator.ProjectOptions{Dir: dir, Name: name, Module: module}
	for _, opt := range opts {
		opt(&options)
	}

	report, err := generator.InitProject(options)
	require.NoError(t, err)

	return report, dir
}

func TestInitProjectGolden(t *testing.T) {
	t.Parallel()

	report, dir := initProject(t, "testapp", "example.com/testapp")
	require.NotEmpty(t, report.Changes)

	for _, change := range report.Changes {
		assert.Equal(t, generator.ActionCreated, change.Action)
		assert.Equal(t, filepath.Join(dir, filepath.FromSlash(change.Rel)), change.Path)

		content, err := os.ReadFile(change.Path)
		require.NoError(t, err)
		compareGolden(t, goldenPath("project", change.Rel), content)
	}

	// The layout the brief promises must exist.
	for _, path := range []string{
		"cmd/api/main.go",
		"internal/features/doc.go",
		"internal/platform/router/router.go",
		"internal/platform/container/container.go",
		"internal/shared/httpx/httpx.go",
		"internal/shared/validator/validator.go",
		"feather.yaml",
		"go.mod",
		".golangci.yml",
		"Makefile",
		".gitignore",
		"README.md",
	} {
		assert.True(t, fileExists(filepath.Join(dir, filepath.FromSlash(path))), "expected %s to exist", path)
	}
}

func TestInitProjectRefusesNonEmptyDirectory(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "testapp")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "existing.go"), []byte("package existing\n"), 0o644))

	_, err := generator.InitProject(generator.ProjectOptions{Dir: dir, Name: "testapp", Module: "example.com/testapp"})
	require.Error(t, err)

	var conflict *generator.ConflictError
	require.ErrorAs(t, err, &conflict)
	assert.Equal(t, "Directory", conflict.Kind)
	assert.Contains(t, err.Error(), "--force")

	// Nothing may be written before the conflict is reported.
	assert.False(t, fileExists(filepath.Join(dir, "go.mod")))
}

func TestInitProjectForceOverwrites(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "testapp")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("stale"), 0o644))

	report, err := generator.InitProject(generator.ProjectOptions{
		Dir: dir, Name: "testapp", Module: "example.com/testapp", Force: true,
	})
	require.NoError(t, err)
	overwritten := report.ByAction(generator.ActionOverwritten)
	require.Len(t, overwritten, 1)
	assert.Equal(t, "go.mod", overwritten[0].Rel)

	content, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "module example.com/testapp")
}

func TestInitProjectDryRunWritesNothing(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "testapp")
	report, err := generator.InitProject(generator.ProjectOptions{
		Dir: dir, Name: "testapp", Module: "example.com/testapp", DryRun: true,
	})
	require.NoError(t, err)
	assert.True(t, report.DryRun)
	assert.NotEmpty(t, report.Changes)
	assert.False(t, fileExists(dir), "a dry run must not create the directory")
}

func TestInitProjectRequiresModuleAndName(t *testing.T) {
	t.Parallel()

	_, err := generator.InitProject(generator.ProjectOptions{Dir: t.TempDir(), Name: "testapp"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "module")

	_, err = generator.InitProject(generator.ProjectOptions{Dir: t.TempDir(), Module: "example.com/testapp"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestInitProjectRejectsFileTarget(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "testapp")
	require.NoError(t, os.WriteFile(dir, []byte("not a directory"), 0o644))

	_, err := generator.InitProject(generator.ProjectOptions{Dir: dir, Name: "testapp", Module: "example.com/testapp"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a directory")
}

// fileExists reports whether path exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
