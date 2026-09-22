package generator

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cybersafetyid/featherctl/internal/config"
	"github.com/cybersafetyid/featherctl/internal/fsutil"
	"github.com/cybersafetyid/featherctl/internal/wiring"
	"github.com/cybersafetyid/featherctl/templates"
)

// FeatureOptions configures [NewFeature].
type FeatureOptions struct {
	// Root is the project root: the directory that contains feather.yaml.
	Root string
	// Name is the feature name exactly as the user typed it.
	Name string
	// Force overwrites the files of an existing feature.
	Force bool
	// DryRun reports what would happen without touching the disk.
	DryRun bool
}

// RemoveFeatureOptions configures [RemoveFeature].
type RemoveFeatureOptions struct {
	// Root is the project root: the directory that contains feather.yaml.
	Root string
	// Name is the feature name exactly as the user typed it.
	Name string
	// DryRun reports what would happen without touching the disk.
	DryRun bool
}

// NotFoundError reports that something featherctl was asked to work on does
// not exist.
type NotFoundError struct {
	// Kind describes what is missing, for example "Feature".
	Kind string
	// Name is the user-facing name of the thing, when there is one.
	Name string
	// Rel is the missing path, relative to the project root.
	Rel string
	// Hint tells the user how to proceed.
	Hint string
}

// Error implements the error interface.
func (e *NotFoundError) Error() string {
	subject := e.Kind
	if e.Name != "" {
		subject += " " + strconv.Quote(e.Name)
	}
	message := fmt.Sprintf("%s not found at %s/", subject, e.Rel)
	if e.Hint != "" {
		message += "\n  " + e.Hint
	}
	return message
}

// featureTemplateData is the value every feature template is executed with.
type featureTemplateData struct {
	// Name is the feature name used in prose, for example "user-profile".
	Name string
	// Package is the Go package name, for example "userprofile".
	Package string
	// Ident is the exported Go identifier, for example "UserProfile".
	Ident string
	// Route is the URL path segment, for example "user-profile".
	Route string
	// Module is the Go module path of the project.
	Module string
	// HttpxImport is the import path of the shared HTTP helpers.
	HttpxImport string
	// ValidatorImport is the import path of the shared validation helpers.
	ValidatorImport string
}

// featureFiles are the files every feature is made of, in the order they are
// created and reported.
var featureFiles = []string{"model.go", "repository.go", "service.go", "handler.go", "handler_test.go", "service_test.go"}

// NewFeature generates a feature slice and wires it into the project's router
// and container.
//
// Nothing is written when opts.DryRun is set, but the returned [Report]
// describes exactly the same changes.
func NewFeature(opts FeatureOptions) (*Report, error) {
	cfg, err := config.Load(opts.Root)
	if err != nil {
		return nil, err
	}

	name, err := ParseFeatureName(opts.Name)
	if err != nil {
		return nil, err
	}

	report := &Report{Root: cfg.Root, DryRun: opts.DryRun}

	dir := cfg.FeaturePath(name.Package)
	if !opts.Force {
		if err := ensureFeatureDirFree(cfg, name, dir); err != nil {
			return report, err
		}
	}

	data := featureTemplateData{
		Name:            name.Route,
		Package:         name.Package,
		Ident:           name.Ident,
		Route:           name.Route,
		Module:          cfg.Module,
		HttpxImport:     packageImport(cfg, "shared", "httpx"),
		ValidatorImport: packageImport(cfg, "shared", "validator"),
	}

	for _, file := range featureFiles {
		spec := fileSpec{
			template:  templates.Feature + "/" + file + ".tmpl",
			path:      filepath.Join(dir, file),
			data:      data,
			overwrite: opts.Force,
			dryRun:    opts.DryRun,
		}
		if err := writeSpec(spec, report); err != nil {
			return report, err
		}
	}

	if err := wireFeature(cfg, name, opts.DryRun, report); err != nil {
		return report, err
	}

	return report, nil
}

// RemoveFeature deletes a generated feature and reverses the wiring featherctl
// added for it.
//
// The wiring is reversed first: if the injected lines cannot be identified with
// certainty, nothing is deleted and the user is told to remove them by hand.
func RemoveFeature(opts RemoveFeatureOptions) (*Report, error) {
	cfg, err := config.Load(opts.Root)
	if err != nil {
		return nil, err
	}

	name, err := ParseFeatureName(opts.Name)
	if err != nil {
		return nil, err
	}

	dir := cfg.FeaturePath(name.Package)
	if !fsutil.IsDir(dir) {
		return nil, &NotFoundError{
			Kind: "Feature",
			Name: name.Raw,
			Rel:  fsutil.Rel(cfg.Root, dir),
			Hint: "Run `feather new feature " + name.Route + "` to create it.",
		}
	}

	report := &Report{Root: cfg.Root, DryRun: opts.DryRun}

	if err := unwireFeature(cfg, name, opts.DryRun, report); err != nil {
		return report, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return report, fmt.Errorf("read %s: %w", fsutil.Rel(cfg.Root, dir), err)
	}
	for _, entry := range entries {
		report.Add(filepath.Join(dir, entry.Name()), ActionRemoved)
	}

	if !opts.DryRun {
		if err := fsutil.RemoveAll(dir); err != nil {
			return report, err
		}
	}
	report.Add(dir, ActionRemoved)

	return report, nil
}

// ensureFeatureDirFree reports a conflict when the feature already exists.
func ensureFeatureDirFree(cfg *config.Config, name FeatureName, dir string) error {
	if !fsutil.IsDir(dir) {
		return nil
	}

	nonEmpty, err := fsutil.DirHasEntries(dir)
	if err != nil {
		return err
	}
	if !nonEmpty {
		return nil
	}

	return &ConflictError{
		Kind: "Feature",
		Name: name.Raw,
		Rel:  fsutil.Rel(cfg.Root, dir),
		Hint: "Use --force to overwrite, or choose a different name.",
	}
}

// wireFeature registers a feature's routes and constructs its dependencies.
func wireFeature(cfg *config.Config, name FeatureName, dryRun bool, report *Report) error {
	importPath := featureImport(cfg, name.Package)

	routerPath := cfg.RouterPath()
	routerSrc, err := os.ReadFile(routerPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", fsutil.Rel(cfg.Root, routerPath), err)
	}

	routed, err := wiring.Inject(routerSrc, wiring.Injection{
		Path:   routerPath,
		Marker: cfg.Router.Marker,
		Code:   fmt.Sprintf("c.%s.RegisterRoutes(mux)", name.Ident),
	})
	if err != nil {
		return fmt.Errorf("wire routes for feature %q: %w", name.Raw, err)
	}
	if err := writeContent(routerPath, routed, true, dryRun, ActionWiredRoutes, report); err != nil {
		return err
	}

	containerPath := cfg.ContainerPath()
	containerSrc, err := os.ReadFile(containerPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", fsutil.Rel(cfg.Root, containerPath), err)
	}

	withFields, err := wiring.Inject(containerSrc, wiring.Injection{
		Path:    containerPath,
		Marker:  cfg.Container.FieldsMarker,
		Code:    featureField(name),
		Imports: []string{importPath},
	})
	if err != nil {
		return fmt.Errorf("add the %q dependency to the container: %w", name.Raw, err)
	}

	withWiring, err := wiring.Inject(withFields, wiring.Injection{
		Path:    containerPath,
		Marker:  cfg.Container.WiringMarker,
		Code:    featureWiring(name),
		Imports: []string{importPath},
	})
	if err != nil {
		return fmt.Errorf("wire the %q dependency into the container: %w", name.Raw, err)
	}

	return writeContent(containerPath, withWiring, true, dryRun, ActionWiredContainer, report)
}

// unwireFeature removes the routes and dependencies featherctl added for a
// feature.
func unwireFeature(cfg *config.Config, name FeatureName, dryRun bool, report *Report) error {
	targets := []struct {
		path  string
		lines []string
	}{
		{path: cfg.RouterPath(), lines: []string{fmt.Sprintf("c.%s.RegisterRoutes(mux)", name.Ident)}},
		{path: cfg.ContainerPath(), lines: append(featureFieldLines(name), featureWiringLines(name)...)},
	}

	for _, target := range targets {
		src, err := os.ReadFile(target.path)
		if err != nil {
			return fmt.Errorf("read %s: %w", fsutil.Rel(cfg.Root, target.path), err)
		}

		out, err := wiring.Remove(src, wiring.Removal{Path: target.path, Lines: target.lines})
		if err != nil {
			return fmt.Errorf(
				"could not un-wire feature %q automatically: %w\n  Remove the injected lines from %s by hand instead",
				name.Raw, err, fsutil.Rel(cfg.Root, target.path),
			)
		}

		if err := writeContent(target.path, out, true, dryRun, ActionModified, report); err != nil {
			return err
		}
	}

	return nil
}

// featureField renders the container struct field for a feature, including its
// doc comment.
func featureField(name FeatureName) string {
	return strings.Join(featureFieldLines(name), "\n")
}

// featureFieldLines is the same code as [featureField], one line at a time,
// which is what un-wiring needs.
func featureFieldLines(name FeatureName) []string {
	return []string{
		fmt.Sprintf("// %s handles HTTP requests for the %s feature.", name.Ident, name.Route),
		fmt.Sprintf("%s *%s.Handler", name.Ident, name.Package),
	}
}

// featureWiring renders the container constructor lines for a feature.
func featureWiring(name FeatureName) string {
	return strings.Join(featureWiringLines(name), "\n")
}

// featureWiringLines is the same code as [featureWiring], one line at a time,
// which is what un-wiring needs.
func featureWiringLines(name FeatureName) []string {
	pkg := name.Package
	return []string{
		fmt.Sprintf("%sRepo := %s.NewInMemoryRepository()", pkg, pkg),
		fmt.Sprintf("%sService := %s.NewService(%sRepo)", pkg, pkg, pkg),
		fmt.Sprintf("c.%s = %s.NewHandler(%sService, c.Logger)", name.Ident, pkg, pkg),
	}
}

// featureImport is the import path of a generated feature package.
func featureImport(cfg *config.Config, pkg string) string {
	return path.Join(cfg.Module, filepath.ToSlash(cfg.FeaturesDir), pkg)
}

// packageImport is the import path of a package under internal/.
func packageImport(cfg *config.Config, elements ...string) string {
	return path.Join(append([]string{cfg.Module, "internal"}, elements...)...)
}
