// Package e2e_test contains the test that proves feather's core promise: the
// project it scaffolds compiles, the features it generates compile, and the
// whole thing keeps working after a feature is added and removed.
package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGeneratedProject walks the release checklist end to end: init a project,
// add two features, build and test it, run doctor, then remove a feature and
// build again.
func TestGeneratedProject(t *testing.T) {
	if testing.Short() {
		t.Skip("end-to-end test shells out to the Go toolchain; skipped in short mode")
	}
	requireGoToolchain(t)

	work := t.TempDir()
	binary := buildFeather(t, work)

	// 1. `feather init testapp` produces a project that compiles.
	run(t, work, binary, "init", "testapp", "--module", "example.com/testapp")
	app := filepath.Join(work, "testapp")
	run(t, app, "go", "build", "./...")
	run(t, app, "go", "vet", "./...")

	// 2. Two features can be generated, and both end up wired in.
	run(t, app, binary, "new", "feature", "order")
	run(t, app, binary, "new", "feature", "user-profile")

	container := readFile(t, filepath.Join(app, "internal", "platform", "container", "container.go"))
	assert.Contains(t, container, "c.Order = order.NewHandler(orderService, c.Logger)")
	assert.Contains(t, container, "c.UserProfile = userprofile.NewHandler(userprofileService, c.Logger)")

	router := readFile(t, filepath.Join(app, "internal", "platform", "router", "router.go"))
	assert.Contains(t, router, "c.Order.RegisterRoutes(mux)")
	assert.Contains(t, router, "c.UserProfile.RegisterRoutes(mux)")

	// 3. The generated project compiles and its generated tests pass.
	run(t, app, "go", "build", "./...")
	run(t, app, "go", "test", "./...")

	// 4. doctor is happy with a healthy project.
	run(t, app, binary, "doctor")
	assert.False(t, fails(t, app, binary, "doctor"), "doctor must pass on a healthy project")

	// 5. Removing a feature un-wires it and leaves the project compiling.
	run(t, app, binary, "remove", "feature", "user-profile")
	run(t, app, "go", "build", "./...")
	run(t, app, "go", "test", "./...")

	remaining := readFile(t, filepath.Join(app, "internal", "platform", "container", "container.go"))
	assert.Contains(t, remaining, "c.Order = order.NewHandler(orderService, c.Logger)")
	assert.NotContains(t, remaining, "userprofile")

	// 6. doctor flags a project whose marker was deleted by hand.
	removeMarker(t, filepath.Join(app, "internal", "platform", "router", "router.go"),
		"// feather:register-routes (do not remove this comment)")

	// The marker is only a warning, so doctor still passes; replacing the whole
	// router with something unreadable is what must fail.
	assert.False(t, fails(t, app, binary, "doctor"))
	assert.Contains(t, run(t, app, binary, "doctor"), "missing")

	require.NoError(t, os.WriteFile(filepath.Join(app, "internal", "platform", "router", "router.go"),
		[]byte("package router\n\nfunc New( {}\n"), 0o644))
	assert.True(t, fails(t, app, binary, "doctor"), "doctor must fail when the project does not compile")
}

// buildFeather compiles the CLI into work and returns the binary path.
func buildFeather(t *testing.T, work string) string {
	t.Helper()

	binary := filepath.Join(work, executableName("feather"))
	run(t, repoRoot(t), "go", "build", "-o", binary, "./cmd/feather")
	return binary
}

// run executes a command in dir and fails the test when it exits non-zero.
func run(t *testing.T, dir, name string, args ...string) string {
	t.Helper()

	output, err := runRaw(t, dir, name, args...)
	require.NoError(t, err, "%s %s failed in %s:\n%s", name, strings.Join(args, " "), dir, output)
	return output
}

// fails executes a command and reports whether it exited non-zero.
func fails(t *testing.T, dir, name string, args ...string) bool {
	t.Helper()

	output, err := runRaw(t, dir, name, args...)
	t.Logf("%s %s (dir %s):\n%s", name, strings.Join(args, " "), dir, output)
	return err != nil
}

// runRaw executes a command and returns its combined output.
func runRaw(t *testing.T, dir, name string, args ...string) (string, error) {
	t.Helper()

	cmd := exec.Command(name, args...)
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	return string(out), err
}

// requireGoToolchain skips the test when the Go toolchain is unavailable.
func requireGoToolchain(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("the go toolchain is not on PATH: %v", err)
	}
}

// repoRoot walks up from the test's directory to the module root.
func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	require.NoError(t, err)

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod found in any parent directory")
		}
		dir = parent
	}
}

// executableName appends the platform's executable suffix.
func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// readFile loads a generated file.
func readFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}

// removeMarker deletes the first line equal to marker from path.
func removeMarker(t *testing.T, path, marker string) {
	t.Helper()

	lines := strings.Split(readFile(t, path), "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == marker {
			continue
		}
		kept = append(kept, line)
	}

	require.NoError(t, os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0o644))
}
