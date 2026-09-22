package generator

import (
	"context"
	"fmt"
	"os"

	"github.com/cybersafetyid/featherctl/internal/config"
	"github.com/cybersafetyid/featherctl/internal/fsutil"
	"github.com/cybersafetyid/featherctl/internal/wiring"
)

// Status is the outcome of a single doctor check.
type Status int

const (
	// StatusOK means the check passed.
	StatusOK Status = iota
	// StatusWarn means the project works but something needs attention.
	StatusWarn
	// StatusFail means the project is broken.
	StatusFail
)

// String renders the status for humans.
func (s Status) String() string {
	switch s {
	case StatusOK:
		return "ok"
	case StatusWarn:
		return "warn"
	case StatusFail:
		return "fail"
	default:
		return "unknown"
	}
}

// Check is the result of one doctor check.
type Check struct {
	// Name is the short label of the check, for example "router".
	Name string
	// Status is the outcome.
	Status Status
	// Message is the one line result.
	Message string
	// Detail is optional extra output, such as a compiler error.
	Detail string
}

// DoctorOptions configures [Doctor].
type DoctorOptions struct {
	// Dir is where to start looking for the project root.
	Dir string
	// SkipBuild skips the `go build ./...` check.
	SkipBuild bool
}

// Failed reports whether any check failed.
func Failed(checks []Check) bool {
	for _, check := range checks {
		if check.Status == StatusFail {
			return true
		}
	}
	return false
}

// Doctor inspects a project and returns one check per validation.
//
// Problems a user can fix by hand (a missing marker, a missing features
// directory) are warnings; anything that stops the project from working at all
// is a failure.
func Doctor(ctx context.Context, opts DoctorOptions) []Check {
	dir := opts.Dir
	if dir == "" {
		dir = "."
	}

	root, err := config.Find(dir)
	if err != nil {
		return []Check{{
			Name:    "project",
			Status:  StatusFail,
			Message: err.Error(),
			Detail:  "Run `feather init <name>` to create a project first.",
		}}
	}

	cfg, err := config.Load(root)
	if err != nil {
		return []Check{{
			Name:    config.FileName,
			Status:  StatusFail,
			Message: err.Error(),
		}}
	}

	checks := []Check{{
		Name:    config.FileName,
		Status:  StatusOK,
		Message: fmt.Sprintf("valid, module %s", cfg.Module),
	}}

	checks = append(checks, markerChecks(cfg)...)
	checks = append(checks, featuresCheck(cfg))

	if !opts.SkipBuild {
		checks = append(checks, buildCheck(ctx, cfg.Root))
	}

	return checks
}

// markerChecks verifies that every injection marker is still present.
func markerChecks(cfg *config.Config) []Check {
	targets := []struct {
		label  string
		path   string
		marker string
	}{
		{"router", cfg.RouterPath(), cfg.Router.Marker},
		{"container fields", cfg.ContainerPath(), cfg.Container.FieldsMarker},
		{"container wiring", cfg.ContainerPath(), cfg.Container.WiringMarker},
	}

	checks := make([]Check, 0, len(targets))
	for _, target := range targets {
		rel := fsutil.Rel(cfg.Root, target.path)

		src, err := os.ReadFile(target.path)
		if err != nil {
			checks = append(checks, Check{
				Name:    "marker " + target.label,
				Status:  StatusFail,
				Message: fmt.Sprintf("cannot read %s", rel),
				Detail:  err.Error(),
			})
			continue
		}

		switch count := wiring.CountMarker(src, target.marker); {
		case count == 1:
			checks = append(checks, Check{
				Name:    "marker " + target.label,
				Status:  StatusOK,
				Message: fmt.Sprintf("found in %s", rel),
			})
		case count == 0:
			checks = append(checks, Check{
				Name:    "marker " + target.label,
				Status:  StatusWarn,
				Message: fmt.Sprintf("missing from %s", rel),
				Detail: "Restore the line " + target.marker + "\n" +
					"or `feather new feature` will refuse to run.",
			})
		default:
			checks = append(checks, Check{
				Name:    "marker " + target.label,
				Status:  StatusWarn,
				Message: fmt.Sprintf("appears %d times in %s", count, rel),
				Detail:  "Keep exactly one copy so injections stay unambiguous.",
			})
		}
	}

	return checks
}

// featuresCheck verifies that the features directory exists.
func featuresCheck(cfg *config.Config) Check {
	dir := cfg.FeaturesPath()
	rel := fsutil.Rel(cfg.Root, dir)

	if fsutil.IsDir(dir) {
		return Check{Name: "features", Status: StatusOK, Message: rel + " exists"}
	}

	return Check{
		Name:    "features",
		Status:  StatusWarn,
		Message: rel + " is missing",
		Detail:  "`feather new feature <name>` creates it for you.",
	}
}

// buildCheck compiles the project.
func buildCheck(ctx context.Context, root string) Check {
	output, err := Build(ctx, root)
	if err == nil {
		return Check{Name: "build", Status: StatusOK, Message: "go build ./... succeeded"}
	}

	detail := output
	if detail == "" {
		detail = err.Error()
	}

	return Check{
		Name:    "build",
		Status:  StatusFail,
		Message: "go build ./... failed",
		Detail:  detail,
	}
}
