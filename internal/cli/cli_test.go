package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cybersafetyid/featherctl/internal/config"
)

// withWorkingDirectory points the commands at dir for the duration of the test.
//
// getwd is a package level hook, so tests that use it must not run in parallel.
func withWorkingDirectory(t *testing.T, dir string) {
	t.Helper()

	original := getwd
	getwd = func() (string, error) { return dir, nil }
	t.Cleanup(func() { getwd = original })
}

// execute runs the command tree and returns its output streams and error.
func execute(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	root := NewRootCommand("test")
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	// An empty, non-terminal input keeps the interactive prompts out of the
	// way: commands must never block or guess in a script.
	root.SetIn(strings.NewReader(""))
	root.SetArgs(args)

	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

func TestVersionFlag(t *testing.T) {
	t.Parallel()

	stdout, _, err := execute(t, "--version")
	require.NoError(t, err)
	assert.Equal(t, "feather test\n", stdout)
}

func TestInitCreatesAProject(t *testing.T) {
	work := t.TempDir()
	withWorkingDirectory(t, work)

	stdout, _, err := execute(t, "init", "demo", "--module", "example.com/demo")
	require.NoError(t, err)

	assert.Contains(t, stdout, "✔ Created go.mod")
	assert.Contains(t, stdout, "✔ Created internal/platform/router/router.go")
	assert.Contains(t, stdout, `Project "demo" is ready.`)
	assert.Contains(t, stdout, "feather new feature order")

	assert.True(t, fileExists(filepath.Join(work, "demo", "feather.yaml")))
}

func TestInitShowsTheFutureTenseOnDryRun(t *testing.T) {
	work := t.TempDir()
	withWorkingDirectory(t, work)

	stdout, _, err := execute(t, "init", "demo", "--module", "example.com/demo", "--dry-run")
	require.NoError(t, err)

	assert.Contains(t, stdout, "✔ Would create go.mod")
	assert.Contains(t, stdout, "⚠ Dry run: nothing was written")
	assert.False(t, fileExists(filepath.Join(work, "demo")))
}

func TestInitRequiresAModulePathWhenNotInteractive(t *testing.T) {
	work := t.TempDir()
	withWorkingDirectory(t, work)

	_, _, err := execute(t, "init", "demo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--module")
}

func TestInitRefusesANonEmptyDirectory(t *testing.T) {
	work := t.TempDir()
	withWorkingDirectory(t, work)

	dir := filepath.Join(work, "demo")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("mine"), 0o644))

	_, _, err := execute(t, "init", "demo", "--module", "example.com/demo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--force")
	assert.False(t, fileExists(filepath.Join(dir, "go.mod")))
}

func TestInitForceOverwritesANonEmptyDirectory(t *testing.T) {
	work := t.TempDir()
	withWorkingDirectory(t, work)

	dir := filepath.Join(work, "demo")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("stale\n"), 0o644))

	stdout, _, err := execute(t, "init", "demo", "--module", "example.com/demo", "--force")
	require.NoError(t, err)
	assert.Contains(t, stdout, "✔ Overwrote go.mod")

	content, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "module example.com/demo")
}

func TestInitValidateCompiles(t *testing.T) {
	work := t.TempDir()
	withWorkingDirectory(t, work)

	stdout, _, err := execute(t, "init", "demo", "--module", "example.com/demo", "--validate")
	require.NoError(t, err)
	assert.Contains(t, stdout, "✔ go build ./... succeeded")
}

func TestNewFeatureForceOverwrites(t *testing.T) {
	work := t.TempDir()
	withWorkingDirectory(t, work)

	_, _, err := execute(t, "init", "demo", "--module", "example.com/demo")
	require.NoError(t, err)

	project := filepath.Join(work, "demo")
	withWorkingDirectory(t, project)

	_, _, err = execute(t, "new", "feature", "order")
	require.NoError(t, err)

	stdout, _, err := execute(t, "new", "feature", "order", "--force")
	require.NoError(t, err)
	assert.Contains(t, stdout, "✔ Overwrote internal/features/order/handler.go")
}

func TestRemoveFeatureReportsMissingFeature(t *testing.T) {
	work := t.TempDir()
	withWorkingDirectory(t, work)

	_, _, err := execute(t, "init", "demo", "--module", "example.com/demo")
	require.NoError(t, err)

	withWorkingDirectory(t, filepath.Join(work, "demo"))

	_, _, err = execute(t, "remove", "feature", "order")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `Feature "order" not found`)
}

func TestDoctorWithoutAProject(t *testing.T) {
	withWorkingDirectory(t, t.TempDir())

	stdout, _, err := execute(t, "doctor", "--skip-build")
	require.ErrorIs(t, err, ErrReported)
	assert.Contains(t, stdout, "✘ project:")
	assert.Contains(t, stdout, config.FileName)
}

func TestNewFeatureCommand(t *testing.T) {
	work := t.TempDir()
	withWorkingDirectory(t, work)

	_, _, err := execute(t, "init", "demo", "--module", "example.com/demo")
	require.NoError(t, err)

	withWorkingDirectory(t, filepath.Join(work, "demo"))

	stdout, _, err := execute(t, "new", "feature", "order")
	require.NoError(t, err)

	assert.Contains(t, stdout, "✔ Created internal/features/order/model.go")
	assert.Contains(t, stdout, "✔ Created internal/features/order/service_test.go")
	assert.Contains(t, stdout, "✔ Wired routes into internal/platform/router/router.go")
	assert.Contains(t, stdout, "✔ Wired container into internal/platform/container/container.go")
	assert.Contains(t, stdout, `Feature "order" is ready. Run `+"`go build ./...`"+` to verify.`)
}

func TestNewFeatureReportsExistingFeature(t *testing.T) {
	work := t.TempDir()
	withWorkingDirectory(t, work)

	_, _, err := execute(t, "init", "demo", "--module", "example.com/demo")
	require.NoError(t, err)

	withWorkingDirectory(t, filepath.Join(work, "demo"))
	_, _, err = execute(t, "new", "feature", "order")
	require.NoError(t, err)

	_, _, err = execute(t, "new", "feature", "order")
	require.Error(t, err)
	assert.Equal(t,
		"Feature \"order\" already exists at internal/features/order/\n  Use --force to overwrite, or choose a different name.",
		err.Error())
}

func TestNewFeatureOutsideAProject(t *testing.T) {
	withWorkingDirectory(t, t.TempDir())

	_, _, err := execute(t, "new", "feature", "order")
	require.Error(t, err)
	assert.Contains(t, err.Error(), config.FileName)
	assert.Contains(t, err.Error(), "feather init")
}

func TestRemoveFeatureCommand(t *testing.T) {
	work := t.TempDir()
	withWorkingDirectory(t, work)

	_, _, err := execute(t, "init", "demo", "--module", "example.com/demo")
	require.NoError(t, err)

	project := filepath.Join(work, "demo")
	withWorkingDirectory(t, project)

	_, _, err = execute(t, "new", "feature", "order")
	require.NoError(t, err)

	stdout, _, err := execute(t, "remove", "feature", "order")
	require.NoError(t, err)

	assert.Contains(t, stdout, "✔ Removed internal/features/order/handler.go")
	assert.Contains(t, stdout, `Feature "order" has been removed.`)
	assert.False(t, fileExists(filepath.Join(project, "internal", "features", "order")))

	container, err := os.ReadFile(filepath.Join(project, "internal", "platform", "container", "container.go"))
	require.NoError(t, err)
	assert.NotContains(t, string(container), "Order *order.Handler")
}

func TestDoctorHealthy(t *testing.T) {
	work := t.TempDir()
	withWorkingDirectory(t, work)

	_, _, err := execute(t, "init", "demo", "--module", "example.com/demo")
	require.NoError(t, err)

	stdout, _, err := execute(t, "doctor", filepath.Join(work, "demo"), "--skip-build")
	require.NoError(t, err)

	assert.Contains(t, stdout, "✔ feather.yaml: valid, module example.com/demo")
	assert.Contains(t, stdout, "Project looks healthy.")
}

func TestDoctorBuilds(t *testing.T) {
	work := t.TempDir()
	withWorkingDirectory(t, work)

	_, _, err := execute(t, "init", "demo", "--module", "example.com/demo")
	require.NoError(t, err)

	stdout, _, err := execute(t, "doctor", filepath.Join(work, "demo"))
	require.NoError(t, err)
	assert.Contains(t, stdout, "✔ build: go build ./... succeeded")
}

func TestDoctorFailsOnBrokenProject(t *testing.T) {
	work := t.TempDir()
	withWorkingDirectory(t, work)

	_, _, err := execute(t, "init", "demo", "--module", "example.com/demo")
	require.NoError(t, err)

	project := filepath.Join(work, "demo")
	require.NoError(t, os.WriteFile(filepath.Join(project, "cmd", "api", "broken.go"),
		[]byte("package main\n\nfunc broken() { undefinedIdentifier }\n"), 0o644))

	stdout, stderr, err := execute(t, "doctor", project)
	require.ErrorIs(t, err, ErrReported)
	assert.Contains(t, stdout, "✘ build: go build ./... failed")
	assert.Contains(t, stderr, "project is not healthy")
	assert.Equal(t, 1, ExitCode(err))
}

func TestUsageErrorsExitWithTwo(t *testing.T) {
	t.Parallel()

	tests := [][]string{
		{"init"},
		{"new", "feature"},
		{"remove", "feature"},
		{"doctor", "a", "b"},
		{"init", "demo", "--nope"},
	}

	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Parallel()

			_, _, err := execute(t, args...)
			require.Error(t, err)

			var usage *UsageError
			assert.ErrorAs(t, err, &usage)
			assert.Equal(t, 2, ExitCode(err))
		})
	}
}

func TestParentCommandsPrintHelp(t *testing.T) {
	t.Parallel()

	stdout, _, err := execute(t, "new")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Usage:")
	assert.Contains(t, stdout, "Generate a feature slice and wire it into the project")
}

// fileExists reports whether path exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
