package generator

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cybersafetyid/featherctl/internal/config"
	"github.com/cybersafetyid/featherctl/internal/fsutil"
	"github.com/cybersafetyid/featherctl/templates"
)

// GoVersion is the Go directive written into a generated go.mod.
const GoVersion = "1.22"

// ConflictError reports an existing path that featherctl refuses to touch
// without --force.
type ConflictError struct {
	// Kind describes what is in the way, for example "Feature" or "Directory".
	Kind string
	// Name is the user-facing name of the thing, when there is one.
	Name string
	// Rel is the conflicting path, relative to the project root.
	Rel string
	// Hint tells the user how to proceed.
	Hint string
}

// Error implements the error interface.
func (e *ConflictError) Error() string {
	subject := e.Kind
	if e.Name != "" {
		subject += " " + strconv.Quote(e.Name)
	}
	return fmt.Sprintf("%s already exists at %s/\n  %s", subject, e.Rel, e.Hint)
}

// ProjectOptions configures [InitProject].
type ProjectOptions struct {
	// Dir is the directory to scaffold, usually <parent>/<name>.
	Dir string
	// Name is the project name used in generated comments and documentation.
	Name string
	// Module is the Go module path. It is required and must not be a guess:
	// featherctl never invents an import path for you.
	Module string
	// Force allows writing into a directory that already contains files.
	Force bool
	// DryRun reports what would be written without touching the disk.
	DryRun bool
}

// projectTemplateData is the value every project template is executed with.
type projectTemplateData struct {
	Name                  string
	Module                string
	GoVersion             string
	FeaturesDir           string
	RouterPath            string
	ContainerPath         string
	RouterMarker          string
	ContainerFieldsMarker string
	ContainerWiringMarker string
}

// InitProject scaffolds a new vertical-slice project in opts.Dir.
//
// It refuses to touch an existing, non-empty directory unless opts.Force is
// set, so it can never quietly overwrite a project that is already there.
func InitProject(opts ProjectOptions) (*Report, error) {
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		return nil, errors.New("project name is required")
	}
	if strings.TrimSpace(opts.Module) == "" {
		return nil, errors.New("a Go module path is required: pass --module github.com/you/myapp")
	}
	if err := validateProjectDir(opts); err != nil {
		return nil, err
	}

	root, err := filepath.Abs(opts.Dir)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", opts.Dir, err)
	}

	data := projectTemplateData{
		Name:                  name,
		Module:                opts.Module,
		GoVersion:             GoVersion,
		FeaturesDir:           config.DefaultFeaturesDir,
		RouterPath:            config.DefaultRouterPath,
		ContainerPath:         config.DefaultContainerPath,
		RouterMarker:          config.DefaultRouterMarker,
		ContainerFieldsMarker: config.DefaultContainerFieldsMarker,
		ContainerWiringMarker: config.DefaultContainerWiringMarker,
	}

	report := &Report{Root: root, DryRun: opts.DryRun}

	specs := []fileSpec{
		{template: templates.Project + "/go.mod.tmpl", path: filepath.Join(root, "go.mod"), data: data},
		{template: templates.Project + "/main.go.tmpl", path: filepath.Join(root, "cmd", "api", "main.go"), data: data},
		{template: templates.Project + "/router.go.tmpl", path: filepath.Join(root, "internal", "platform", "router", "router.go"), data: data},
		{template: templates.Project + "/container.go.tmpl", path: filepath.Join(root, "internal", "platform", "container", "container.go"), data: data},
		{template: templates.Project + "/httpx.go.tmpl", path: filepath.Join(root, "internal", "shared", "httpx", "httpx.go"), data: data},
		{template: templates.Project + "/validator.go.tmpl", path: filepath.Join(root, "internal", "shared", "validator", "validator.go"), data: data},
		{template: templates.Project + "/features_doc.go.tmpl", path: filepath.Join(root, "internal", "features", "doc.go"), data: data},
		{template: templates.Project + "/feather.yaml.tmpl", path: filepath.Join(root, config.FileName), data: data},
		{template: templates.Project + "/golangci.yml.tmpl", path: filepath.Join(root, ".golangci.yml"), data: data},
		{template: templates.Project + "/Makefile.tmpl", path: filepath.Join(root, "Makefile"), data: data},
		{template: templates.Project + "/gitignore.tmpl", path: filepath.Join(root, ".gitignore"), data: data},
		{template: templates.Project + "/README.md.tmpl", path: filepath.Join(root, "README.md"), data: data},
	}

	for _, spec := range specs {
		spec.overwrite = opts.Force
		spec.dryRun = opts.DryRun
		if err := writeSpec(spec, report); err != nil {
			return report, err
		}
	}

	return report, nil
}

// validateProjectDir rejects a target directory that is in the way.
func validateProjectDir(opts ProjectOptions) error {
	info, err := os.Stat(opts.Dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat %s: %w", opts.Dir, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("%s already exists and is not a directory", opts.Dir)
	}

	nonEmpty, err := fsutil.DirHasEntries(opts.Dir)
	if err != nil {
		return err
	}
	if !nonEmpty || opts.Force {
		return nil
	}

	return &ConflictError{
		Kind: "Directory",
		Name: opts.Name,
		Rel:  filepath.ToSlash(opts.Dir),
		Hint: "Use --force to write into it anyway, or choose a different name.",
	}
}
