package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cybersafetyid/featherctl/internal/config"
)

const validYAML = `version: 1
module: example.com/demo
features_dir: internal/features

router:
  path: internal/platform/router/router.go
  marker: "// feather:register-routes (do not remove this comment)"

container:
  path: internal/platform/container/container.go
  fields_marker: "// feather:container-fields (do not remove this comment)"
  wiring_marker: "// feather:container-wiring (do not remove this comment)"
`

func TestParseValidDocument(t *testing.T) {
	t.Parallel()

	cfg, err := config.Parse("/tmp/demo", []byte(validYAML))
	require.NoError(t, err)

	assert.Equal(t, 1, cfg.Version)
	assert.Equal(t, "example.com/demo", cfg.Module)
	assert.Equal(t, "internal/features", cfg.FeaturesDir)
	assert.Equal(t, "internal/platform/router/router.go", cfg.Router.Path)
	assert.Equal(t, config.DefaultRouterMarker, cfg.Router.Marker)
}

func TestParseAppliesDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := config.Parse("/tmp/demo", []byte("module: example.com/demo\n"))
	require.NoError(t, err)

	assert.Equal(t, config.Version, cfg.Version)
	assert.Equal(t, config.DefaultFeaturesDir, cfg.FeaturesDir)
	assert.Equal(t, config.DefaultRouterPath, cfg.Router.Path)
	assert.Equal(t, config.DefaultRouterMarker, cfg.Router.Marker)
	assert.Equal(t, config.DefaultContainerPath, cfg.Container.Path)
	assert.Equal(t, config.DefaultContainerFieldsMarker, cfg.Container.FieldsMarker)
	assert.Equal(t, config.DefaultContainerWiringMarker, cfg.Container.WiringMarker)
}

func TestParseRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	_, err := config.Parse("/tmp/demo", []byte("module: example.com/demo\nmystery: 1\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mystery")
}

func TestParseRejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		document string
		want     string
	}{
		{name: "missing module", document: "version: 1\n", want: "module is required"},
		{name: "blank module", document: "module: \"  \"\n", want: "module is required"},
		{name: "module with whitespace", document: "module: \"example.com/my app\"\n", want: "whitespace"},
		{name: "unsupported version", document: "version: 99\nmodule: example.com/demo\n", want: "unsupported"},
		{name: "absolute features dir", document: "module: example.com/demo\nfeatures_dir: /etc\n", want: "must be relative"},
		{name: "escaping features dir", document: "module: example.com/demo\nfeatures_dir: ../escape\n", want: "must stay inside"},
		{name: "marker is not a comment", document: "module: x\nrouter:\n  marker: feather:register\n", want: "Go line comment"},
		{name: "marker with spaces only", document: "module: x\nrouter:\n  marker: \"  \"\n", want: "router.marker must not be empty"},
		{name: "invalid yaml", document: "module: [oops\n", want: "invalid"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := config.Parse("/tmp/demo", []byte(test.document))
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
		})
	}
}

func TestLoadAndFind(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	nested := filepath.Join(root, "internal", "features", "order")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, config.FileName), []byte(validYAML), 0o644))

	found, err := config.Find(nested)
	require.NoError(t, err)
	assert.Equal(t, root, found)

	cfg, err := config.Load(root)
	require.NoError(t, err)
	assert.Equal(t, root, cfg.Root)
	assert.Equal(t, filepath.Join(root, "internal", "features", "order"), cfg.FeaturePath("order"))
	assert.Equal(t, filepath.Join(root, "internal", "platform", "router", "router.go"), cfg.RouterPath())
	assert.Equal(t, filepath.Join(root, "internal", "platform", "container", "container.go"), cfg.ContainerPath())
}

func TestLoadMissing(t *testing.T) {
	t.Parallel()

	_, err := config.Load(t.TempDir())
	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrNotFound)
	assert.Contains(t, err.Error(), config.FileName)
}

func TestFindMissing(t *testing.T) {
	t.Parallel()

	_, err := config.Find(t.TempDir())
	assert.ErrorIs(t, err, config.ErrNotFound)
}

func TestMarshalRoundTrip(t *testing.T) {
	t.Parallel()

	original, err := config.Parse("/tmp/demo", []byte(validYAML))
	require.NoError(t, err)

	encoded, err := original.Marshal()
	require.NoError(t, err)

	decoded, err := config.Parse("/tmp/demo", encoded)
	require.NoError(t, err)
	assert.Equal(t, original.Module, decoded.Module)
	assert.Equal(t, original.Router, decoded.Router)
	assert.Equal(t, original.Container, decoded.Container)
}
