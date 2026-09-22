package generator

import (
	"github.com/cybersafetyid/featherctl/internal/fsutil"
)

// Action describes what a generator run did to one path.
type Action string

const (
	// ActionCreated means the file did not exist and was written.
	ActionCreated Action = "created"
	// ActionOverwritten means the file existed and --force replaced it.
	ActionOverwritten Action = "overwritten"
	// ActionModified means an existing file was edited in place.
	ActionModified Action = "modified"
	// ActionRemoved means the path was deleted.
	ActionRemoved Action = "removed"
	// ActionWiredRoutes means the file had a feature's routes registered in it.
	ActionWiredRoutes Action = "wired-routes"
	// ActionWiredContainer means the file had a feature's dependencies wired
	// into it.
	ActionWiredContainer Action = "wired-container"
)

// Change is a single path affected by a generator run.
type Change struct {
	// Path is the absolute path of the file.
	Path string
	// Rel is the path relative to the project root, using forward slashes.
	Rel string
	// Action is what happened to the file.
	Action Action
}

// Report summarises everything a generator run did, in the order it happened.
//
// The report is produced even for a dry run, so callers can render exactly the
// same output whether or not anything was written.
type Report struct {
	// Root is the project root the changes are relative to.
	Root string
	// DryRun reports whether the run was a dry run.
	DryRun bool
	// Changes lists every affected path in the order it was handled.
	Changes []Change
}

// Add records one change.
func (r *Report) Add(path string, action Action) {
	r.Changes = append(r.Changes, Change{
		Path:   path,
		Rel:    fsutil.Rel(r.Root, path),
		Action: action,
	})
}

// ByAction returns every change with the given action.
func (r *Report) ByAction(action Action) []Change {
	var out []Change
	for _, change := range r.Changes {
		if change.Action == action {
			out = append(out, change)
		}
	}
	return out
}
