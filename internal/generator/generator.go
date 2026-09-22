// Package generator turns featherctl's embedded templates into a compilable
// vertical-slice project and into new feature slices inside it.
//
// The package never writes to standard output and never calls os.Exit: it
// returns a [Report] describing what it did so the CLI layer stays a thin
// wrapper.
package generator

import (
	"bytes"
	"fmt"
	"io/fs"
	"path"
	"text/template"

	"github.com/cybersafetyid/featherctl/internal/fsutil"
	"github.com/cybersafetyid/featherctl/templates"
)

// fileSpec describes one rendered file.
type fileSpec struct {
	// template is the path inside the embedded template filesystem.
	template string
	// path is the absolute destination path.
	path string
	// data is the value the template is executed with.
	data any
	// overwrite allows replacing a file that already exists.
	overwrite bool
	// dryRun records the change without touching the disk.
	dryRun bool
}

// render executes the template stored at name inside fsys.
func render(fsys fs.FS, name string, data any) ([]byte, error) {
	raw, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, fmt.Errorf("read template %s: %w", name, err)
	}

	tmpl, err := template.New(path.Base(name)).Option("missingkey=error").Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", name, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("render template %s: %w", name, err)
	}
	return buf.Bytes(), nil
}

// writeSpec renders and writes one file, recording the result in report.
func writeSpec(spec fileSpec, report *Report) error {
	content, err := render(templates.FS, spec.template, spec.data)
	if err != nil {
		return err
	}

	action := ActionCreated
	if fsutil.Exists(spec.path) {
		if !spec.overwrite {
			return &fsutil.ConflictError{Path: spec.path}
		}
		action = ActionOverwritten
	}

	if !spec.dryRun {
		if err := fsutil.WriteFile(spec.path, content, spec.overwrite); err != nil {
			return err
		}
	}

	report.Add(spec.path, action)
	return nil
}

// writeContent writes generated content that is not backed by a template
// (wiring edits are produced from an existing file, not rendered).
func writeContent(path string, content []byte, overwrite, dryRun bool, action Action, report *Report) error {
	if !dryRun {
		if err := fsutil.WriteFile(path, content, overwrite); err != nil {
			return err
		}
	}
	report.Add(path, action)
	return nil
}
