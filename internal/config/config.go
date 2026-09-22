// Package config loads and validates the feather.yaml file that describes how
// a project is laid out and where featherctl is allowed to auto-wire generated
// code.
//
// The package is deliberately free of any CLI concerns so it can be unit
// tested in isolation and reused by other tools.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// FileName is the name of the configuration file that marks the root of a
// featherctl-managed project.
const FileName = "feather.yaml"

// Version is the configuration schema version understood by this build.
const Version = 1

// Default values applied when a field is omitted from feather.yaml. They match
// the layout produced by `feather init`.
const (
	// DefaultFeaturesDir is the directory that holds one package per feature.
	DefaultFeaturesDir = "internal/features"
	// DefaultRouterPath is the file that registers feature routes.
	DefaultRouterPath = "internal/platform/router/router.go"
	// DefaultRouterMarker is the comment that marks the route injection point.
	DefaultRouterMarker = "// feather:register-routes (do not remove this comment)"
	// DefaultContainerPath is the file that wires feature dependencies.
	DefaultContainerPath = "internal/platform/container/container.go"
	// DefaultContainerFieldsMarker marks where feature dependencies are added
	// to the Container struct.
	DefaultContainerFieldsMarker = "// feather:container-fields (do not remove this comment)"
	// DefaultContainerWiringMarker marks where feature dependencies are
	// constructed inside the container constructor.
	DefaultContainerWiringMarker = "// feather:container-wiring (do not remove this comment)"
)

// ErrNotFound is returned by Find and Load when no feather.yaml can be located.
var ErrNotFound = errors.New("no " + FileName + " found in this directory or any parent directory")

// Router describes the file that registers feature routes on the HTTP router.
type Router struct {
	// Path is the router file, relative to the project root.
	Path string `yaml:"path"`
	// Marker is the comment featherctl inserts feature routes above.
	Marker string `yaml:"marker"`
}

// Container describes the file that constructs and holds feature dependencies.
type Container struct {
	// Path is the container file, relative to the project root.
	Path string `yaml:"path"`
	// FieldsMarker is the comment featherctl inserts feature dependency fields
	// at, inside the container struct.
	FieldsMarker string `yaml:"fields_marker"`
	// WiringMarker is the comment featherctl inserts feature construction code
	// at, inside the container constructor.
	WiringMarker string `yaml:"wiring_marker"`
}

// Config is the parsed representation of a feather.yaml file.
type Config struct {
	// Version is the configuration schema version. It defaults to [Version].
	Version int `yaml:"version"`
	// Module is the Go module path of the project, used to build import paths
	// for generated code. It is required.
	Module string `yaml:"module"`
	// FeaturesDir is the directory that holds one package per feature.
	FeaturesDir string `yaml:"features_dir"`
	// Router points at the router file and its injection marker.
	Router Router `yaml:"router"`
	// Container points at the container file and its injection markers.
	Container Container `yaml:"container"`

	// Root is the absolute path of the directory containing feather.yaml. It is
	// never read from or written to the YAML document.
	Root string `yaml:"-"`
}

// Load reads and validates feather.yaml from root.
//
// It returns [ErrNotFound] wrapped with the offending path when the file is
// missing.
func Load(root string) (*Config, error) {
	path := filepath.Join(root, FileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w (looked in %s)", ErrNotFound, path)
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	cfg, err := Parse(root, data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

// Parse decodes a feather.yaml document, applies defaults for omitted fields
// and validates the result. root is the directory the document was read from;
// it is used to resolve the paths the config refers to.
func Parse(root string, data []byte) (*Config, error) {
	var cfg Config
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", FileName, err)
	}

	cfg.Root = root
	cfg.applyDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// applyDefaults fills in every field that is optional in the YAML document.
func (c *Config) applyDefaults() {
	if c.Version == 0 {
		c.Version = Version
	}
	if c.FeaturesDir == "" {
		c.FeaturesDir = DefaultFeaturesDir
	}
	if c.Router.Path == "" {
		c.Router.Path = DefaultRouterPath
	}
	if c.Router.Marker == "" {
		c.Router.Marker = DefaultRouterMarker
	}
	if c.Container.Path == "" {
		c.Container.Path = DefaultContainerPath
	}
	if c.Container.FieldsMarker == "" {
		c.Container.FieldsMarker = DefaultContainerFieldsMarker
	}
	if c.Container.WiringMarker == "" {
		c.Container.WiringMarker = DefaultContainerWiringMarker
	}
}

// Validate reports whether the configuration is usable. The returned error
// explains exactly which field is wrong and how to fix it.
func (c *Config) Validate() error {
	if c.Version != Version {
		return fmt.Errorf("unsupported %s version %d, this build supports version %d", FileName, c.Version, Version)
	}

	if strings.TrimSpace(c.Module) == "" {
		return fmt.Errorf("module is required in %s (for example: module: github.com/you/myapp)", FileName)
	}
	if strings.ContainsAny(c.Module, " \t") {
		return fmt.Errorf("module %q must not contain whitespace", c.Module)
	}

	paths := []struct {
		field string
		value string
	}{
		{"features_dir", c.FeaturesDir},
		{"router.path", c.Router.Path},
		{"container.path", c.Container.Path},
	}
	for _, p := range paths {
		if err := validateRelativePath(p.field, p.value); err != nil {
			return err
		}
	}

	markers := []struct {
		field string
		value string
	}{
		{"router.marker", c.Router.Marker},
		{"container.fields_marker", c.Container.FieldsMarker},
		{"container.wiring_marker", c.Container.WiringMarker},
	}
	for _, m := range markers {
		if strings.TrimSpace(m.value) == "" {
			return fmt.Errorf("%s must not be empty", m.field)
		}
		if !strings.HasPrefix(strings.TrimSpace(m.value), "//") {
			return fmt.Errorf("%s must be a Go line comment starting with %q, got %q", m.field, "//", m.value)
		}
	}

	return nil
}

// validateRelativePath rejects empty, absolute and root-escaping paths.
//
// Paths are normalised with forward slashes first so that a configuration file
// behaves the same on Windows as it does on Linux and macOS.
func validateRelativePath(field, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("%s must not be empty", field)
	}

	// filepath.IsAbs is platform dependent, so also reject a leading slash:
	// "/etc" is absolute everywhere even where the platform disagrees.
	if filepath.IsAbs(trimmed) || strings.HasPrefix(trimmed, "/") {
		return fmt.Errorf("%s must be relative to the project root, got absolute path %q", field, value)
	}

	cleaned := filepath.ToSlash(filepath.Clean(filepath.FromSlash(trimmed)))
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return fmt.Errorf("%s must stay inside the project root, got %q", field, value)
	}

	return nil
}

// Find walks from dir towards the filesystem root looking for feather.yaml and
// returns the directory that contains it.
func Find(dir string) (string, error) {
	current, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", dir, err)
	}

	for {
		if info, err := os.Stat(filepath.Join(current, FileName)); err == nil && !info.IsDir() {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", ErrNotFound
		}
		current = parent
	}
}

// FeaturesPath returns the absolute path of the features directory.
func (c *Config) FeaturesPath() string {
	return c.abs(c.FeaturesDir)
}

// FeaturePath returns the absolute path of the package directory for name.
func (c *Config) FeaturePath(name string) string {
	return filepath.Join(c.FeaturesPath(), name)
}

// RouterPath returns the absolute path of the router file.
func (c *Config) RouterPath() string {
	return c.abs(c.Router.Path)
}

// ContainerPath returns the absolute path of the container file.
func (c *Config) ContainerPath() string {
	return c.abs(c.Container.Path)
}

// abs joins a project-relative path onto the project root.
func (c *Config) abs(rel string) string {
	if c.Root == "" {
		return filepath.Clean(rel)
	}
	return filepath.Join(c.Root, filepath.FromSlash(rel))
}

// Marshal renders the configuration as a YAML document.
func (c *Config) Marshal() ([]byte, error) {
	data, err := yaml.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", FileName, err)
	}
	return data, nil
}
