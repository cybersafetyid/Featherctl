package generator_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cybersafetyid/featherctl/internal/config"
	"github.com/cybersafetyid/featherctl/internal/generator"
)

// newProject scaffolds a project and returns its root.
func newProject(t *testing.T) string {
	t.Helper()

	_, dir := initProject(t, "testapp", "example.com/testapp")
	return dir
}

// readFile loads a generated file.
func readFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}

// requireParses asserts src is valid Go.
func requireParses(t *testing.T, src string) {
	t.Helper()

	_, err := parser.ParseFile(token.NewFileSet(), "generated.go", src, parser.ParseComments)
	require.NoError(t, err, "generated file must parse:\n%s", src)
}

func TestNewFeatureGolden(t *testing.T) {
	t.Parallel()

	root := newProject(t)

	report, err := generator.NewFeature(generator.FeatureOptions{Root: root, Name: "user-profile"})
	require.NoError(t, err)

	created := report.ByAction(generator.ActionCreated)
	require.Len(t, created, 6)
	for _, change := range created {
		compareGolden(t, goldenPath("feature", filepath.Base(change.Rel)), []byte(readFile(t, change.Path)))
	}

	for _, change := range report.ByAction(generator.ActionWiredRoutes) {
		compareGolden(t, goldenPath("wired", filepath.Base(change.Rel)), []byte(readFile(t, change.Path)))
	}
	for _, change := range report.ByAction(generator.ActionWiredContainer) {
		compareGolden(t, goldenPath("wired", filepath.Base(change.Rel)), []byte(readFile(t, change.Path)))
	}
}

func TestNewFeatureWiresContainerAndRouter(t *testing.T) {
	t.Parallel()

	root := newProject(t)

	_, err := generator.NewFeature(generator.FeatureOptions{Root: root, Name: "order"})
	require.NoError(t, err)

	container := readFile(t, filepath.Join(root, "internal/platform/container/container.go"))
	requireParses(t, container)

	assert.Contains(t, container, `"example.com/testapp/internal/features/order"`)
	assert.Contains(t, container, "Order *order.Handler")
	assert.Contains(t, container, "orderRepo := order.NewInMemoryRepository()")
	assert.Contains(t, container, "c.Order = order.NewHandler(orderService, c.Logger)")
	assert.Equal(t, 1, strings.Count(container, config.DefaultContainerWiringMarker))

	router := readFile(t, filepath.Join(root, "internal/platform/router/router.go"))
	requireParses(t, router)
	assert.Contains(t, router, "c.Order.RegisterRoutes(mux)")
	assert.Equal(t, 1, strings.Count(router, config.DefaultRouterMarker))
}

func TestNewFeatureTwiceKeepsBothWired(t *testing.T) {
	t.Parallel()

	root := newProject(t)

	for _, name := range []string{"order", "user-profile"} {
		_, err := generator.NewFeature(generator.FeatureOptions{Root: root, Name: name})
		require.NoError(t, err)
	}

	container := readFile(t, filepath.Join(root, "internal/platform/container/container.go"))
	requireParses(t, container)

	assert.Contains(t, container, "Order *order.Handler")
	assert.Contains(t, container, "UserProfile *userprofile.Handler")
	assert.Contains(t, container, "c.Order = order.NewHandler(orderService, c.Logger)")
	assert.Contains(t, container, "c.UserProfile = userprofile.NewHandler(userprofileService, c.Logger)")

	router := readFile(t, filepath.Join(root, "internal/platform/router/router.go"))
	requireParses(t, router)

	assert.Contains(t, router, "c.Order.RegisterRoutes(mux)")
	assert.Contains(t, router, "c.UserProfile.RegisterRoutes(mux)")

	// Feature order must not leak into the wiring section.
	assert.Less(t,
		strings.Index(container, "c.Order = "),
		strings.Index(container, "c.UserProfile = "))
}

func TestNewFeatureRefusesExistingFeature(t *testing.T) {
	t.Parallel()

	root := newProject(t)
	_, err := generator.NewFeature(generator.FeatureOptions{Root: root, Name: "order"})
	require.NoError(t, err)

	_, err = generator.NewFeature(generator.FeatureOptions{Root: root, Name: "order"})
	require.Error(t, err)

	var conflict *generator.ConflictError
	require.ErrorAs(t, err, &conflict)
	assert.Equal(t, "Feature", conflict.Kind)
	assert.Contains(t, err.Error(), `Feature "order" already exists at internal/features/order/`)
	assert.Contains(t, err.Error(), "Use --force to overwrite, or choose a different name.")
}

func TestNewFeatureForceRecreatesFiles(t *testing.T) {
	t.Parallel()

	root := newProject(t)
	_, err := generator.NewFeature(generator.FeatureOptions{Root: root, Name: "order"})
	require.NoError(t, err)

	handler := filepath.Join(root, "internal/features/order/handler.go")
	require.NoError(t, os.WriteFile(handler, []byte("package order\n"), 0o644))

	report, err := generator.NewFeature(generator.FeatureOptions{Root: root, Name: "order", Force: true})
	require.NoError(t, err)
	assert.Len(t, report.ByAction(generator.ActionOverwritten), 6)
	assert.Contains(t, readFile(t, handler), "func (h *Handler) RegisterRoutes")
}

func TestNewFeatureDryRunWritesNothing(t *testing.T) {
	t.Parallel()

	root := newProject(t)
	router := filepath.Join(root, "internal/platform/router/router.go")
	before := readFile(t, router)

	report, err := generator.NewFeature(generator.FeatureOptions{Root: root, Name: "order", DryRun: true})
	require.NoError(t, err)

	assert.True(t, report.DryRun)
	assert.Len(t, report.Changes, 8, "six files plus the two wired files")
	assert.False(t, fileExists(filepath.Join(root, "internal/features/order")))
	assert.Equal(t, before, readFile(t, router), "the router must be untouched")
}

func TestNewFeatureRequiresAProject(t *testing.T) {
	t.Parallel()

	_, err := generator.NewFeature(generator.FeatureOptions{Root: t.TempDir(), Name: "order"})
	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrNotFound)
}

func TestNewFeatureAbortsWhenMarkerIsMissing(t *testing.T) {
	t.Parallel()

	root := newProject(t)
	router := filepath.Join(root, "internal/platform/router/router.go")

	broken := strings.Replace(readFile(t, router), config.DefaultRouterMarker, "// gone", 1)
	require.NoError(t, os.WriteFile(router, []byte(broken), 0o644))

	_, err := generator.NewFeature(generator.FeatureOptions{Root: root, Name: "order"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marker")

	assert.Equal(t, broken, readFile(t, router), "the broken router must be left alone")
}

func TestRemoveFeatureRestoresTheProject(t *testing.T) {
	t.Parallel()

	root := newProject(t)
	router := filepath.Join(root, "internal/platform/router/router.go")
	container := filepath.Join(root, "internal/platform/container/container.go")

	_, err := generator.NewFeature(generator.FeatureOptions{Root: root, Name: "order"})
	require.NoError(t, err)
	_, err = generator.NewFeature(generator.FeatureOptions{Root: root, Name: "user-profile"})
	require.NoError(t, err)

	beforeRouter := readFile(t, router)
	beforeContainer := readFile(t, container)

	report, err := generator.RemoveFeature(generator.RemoveFeatureOptions{Root: root, Name: "user-profile"})
	require.NoError(t, err)
	assert.NotEmpty(t, report.ByAction(generator.ActionRemoved))

	assert.False(t, fileExists(filepath.Join(root, "internal/features/userprofile")))

	afterContainer := readFile(t, container)
	requireParses(t, afterContainer)
	assert.NotContains(t, afterContainer, "userprofile")
	assert.Contains(t, afterContainer, "c.Order = order.NewHandler(orderService, c.Logger)")
	assert.Equal(t, 1, strings.Count(afterContainer, config.DefaultContainerFieldsMarker))
	assert.Equal(t, 1, strings.Count(afterContainer, config.DefaultContainerWiringMarker))

	afterRouter := readFile(t, router)
	requireParses(t, afterRouter)
	assert.NotContains(t, afterRouter, "UserProfile")
	assert.Contains(t, afterRouter, "c.Order.RegisterRoutes(mux)")
	assert.Equal(t, 1, strings.Count(afterRouter, config.DefaultRouterMarker))

	// Removing the second feature must restore exactly the state after the
	// first one was added.
	_, err = generator.RemoveFeature(generator.RemoveFeatureOptions{Root: root, Name: "order"})
	require.NoError(t, err)
	assert.NotEqual(t, beforeRouter, readFile(t, router))
	assert.NotEqual(t, beforeContainer, readFile(t, container))
	assert.False(t, fileExists(filepath.Join(root, "internal/features/order")))
}

func TestRemoveFeatureDryRunWritesNothing(t *testing.T) {
	t.Parallel()

	root := newProject(t)
	_, err := generator.NewFeature(generator.FeatureOptions{Root: root, Name: "order"})
	require.NoError(t, err)

	router := filepath.Join(root, "internal/platform/router/router.go")
	before := readFile(t, router)

	report, err := generator.RemoveFeature(generator.RemoveFeatureOptions{Root: root, Name: "order", DryRun: true})
	require.NoError(t, err)

	assert.True(t, report.DryRun)
	assert.NotEmpty(t, report.ByAction(generator.ActionRemoved))
	assert.True(t, fileExists(filepath.Join(root, "internal/features/order")))
	assert.Equal(t, before, readFile(t, router))
}

func TestRemoveFeatureMissing(t *testing.T) {
	t.Parallel()

	root := newProject(t)
	_, err := generator.RemoveFeature(generator.RemoveFeatureOptions{Root: root, Name: "order"})
	require.Error(t, err)

	var missing *generator.NotFoundError
	require.ErrorAs(t, err, &missing)
	assert.Equal(t, "Feature", missing.Kind)
}

func TestRemoveFeatureRefusesHandEditedWiring(t *testing.T) {
	t.Parallel()

	root := newProject(t)
	_, err := generator.NewFeature(generator.FeatureOptions{Root: root, Name: "order"})
	require.NoError(t, err)

	// A human edited the wiring: featherctl must not guess what to delete.
	router := filepath.Join(root, "internal/platform/router/router.go")
	edited := strings.Replace(readFile(t, router), "c.Order.RegisterRoutes(mux)", "setupOrder(mux)", 1)
	require.NoError(t, os.WriteFile(router, []byte(edited), 0o644))

	_, err = generator.RemoveFeature(generator.RemoveFeatureOptions{Root: root, Name: "order"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "by hand")

	// Nothing was deleted.
	assert.True(t, fileExists(filepath.Join(root, "internal/features/order")))
	assert.Equal(t, edited, readFile(t, router))
}
