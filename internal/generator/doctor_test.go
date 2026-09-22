package generator_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cybersafetyid/featherctl/internal/config"
	"github.com/cybersafetyid/featherctl/internal/generator"
)

// statusOf returns the check with the given name.
func statusOf(t *testing.T, checks []generator.Check, name string) generator.Check {
	t.Helper()

	for _, check := range checks {
		if check.Name == name {
			return check
		}
	}
	t.Fatalf("no check named %q in %+v", name, checks)
	return generator.Check{}
}

func TestDoctorHealthyProject(t *testing.T) {
	t.Parallel()

	root := newProject(t)
	checks := generator.Doctor(context.Background(), generator.DoctorOptions{Dir: root, SkipBuild: true})

	require.NotEmpty(t, checks)
	assert.False(t, generator.Failed(checks))

	assert.Equal(t, generator.StatusOK, statusOf(t, checks, "feather.yaml").Status)
	assert.Equal(t, generator.StatusOK, statusOf(t, checks, "marker router").Status)
	assert.Equal(t, generator.StatusOK, statusOf(t, checks, "marker container fields").Status)
	assert.Equal(t, generator.StatusOK, statusOf(t, checks, "marker container wiring").Status)
	assert.Equal(t, generator.StatusOK, statusOf(t, checks, "features").Status)
}

func TestDoctorWarnsWhenMarkerIsDeleted(t *testing.T) {
	t.Parallel()

	root := newProject(t)
	router := filepath.Join(root, "internal/platform/router/router.go")
	broken := strings.Replace(readFile(t, router), config.DefaultRouterMarker, "// removed by hand", 1)
	require.NoError(t, os.WriteFile(router, []byte(broken), 0o644))

	checks := generator.Doctor(context.Background(), generator.DoctorOptions{Dir: root, SkipBuild: true})

	routerCheck := statusOf(t, checks, "marker router")
	assert.Equal(t, generator.StatusWarn, routerCheck.Status)
	assert.Contains(t, routerCheck.Message, "missing")
	assert.False(t, generator.Failed(checks), "a missing marker is a warning, not a failure")
}

func TestDoctorWarnsWhenMarkerIsDuplicated(t *testing.T) {
	t.Parallel()

	root := newProject(t)
	router := filepath.Join(root, "internal/platform/router/router.go")
	duplicated := strings.Replace(readFile(t, router), config.DefaultRouterMarker,
		config.DefaultRouterMarker+"\n\t"+config.DefaultRouterMarker, 1)
	require.NoError(t, os.WriteFile(router, []byte(duplicated), 0o644))

	checks := generator.Doctor(context.Background(), generator.DoctorOptions{Dir: root, SkipBuild: true})

	routerCheck := statusOf(t, checks, "marker router")
	assert.Equal(t, generator.StatusWarn, routerCheck.Status)
	assert.Contains(t, routerCheck.Message, "2 times")
}

func TestDoctorFailsWithoutAProject(t *testing.T) {
	t.Parallel()

	checks := generator.Doctor(context.Background(), generator.DoctorOptions{Dir: t.TempDir(), SkipBuild: true})

	require.Len(t, checks, 1)
	assert.Equal(t, generator.StatusFail, checks[0].Status)
	assert.True(t, generator.Failed(checks))
}

func TestDoctorFailsOnInvalidConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		document string
		want     string
	}{
		{name: "unsupported version", document: "version: 99\nmodule: example.com/demo\n", want: "unsupported"},
		{name: "missing module", document: "version: 1\n", want: "module is required"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(root, config.FileName), []byte(test.document), 0o644))

			checks := generator.Doctor(context.Background(), generator.DoctorOptions{Dir: root, SkipBuild: true})

			require.Len(t, checks, 1)
			assert.Equal(t, generator.StatusFail, checks[0].Status)
			assert.Contains(t, checks[0].Message, test.want)
		})
	}
}

func TestDoctorBuildCheck(t *testing.T) {
	t.Parallel()

	root := newProject(t)

	healthy := generator.Doctor(context.Background(), generator.DoctorOptions{Dir: root})
	assert.Equal(t, generator.StatusOK, statusOf(t, healthy, "build").Status)
	assert.False(t, generator.Failed(healthy))

	// Break the project and confirm doctor catches it.
	require.NoError(t, os.WriteFile(filepath.Join(root, "cmd/api/broken.go"), []byte("package main\n\nfunc broken() { undefinedIdentifier }\n"), 0o644))

	broken := generator.Doctor(context.Background(), generator.DoctorOptions{Dir: root})
	buildCheck := statusOf(t, broken, "build")
	assert.Equal(t, generator.StatusFail, buildCheck.Status)
	assert.Contains(t, buildCheck.Detail, "undefinedIdentifier")
	assert.True(t, generator.Failed(broken))
}
