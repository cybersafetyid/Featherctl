// Package cli wires feather's command tree.
//
// Every command in this package is deliberately thin: it parses flags, calls
// into [github.com/cybersafetyid/featherctl/internal/generator] and renders the
// report. Business logic belongs in the generator, not here.
package cli

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
)

// ErrReported is returned by a command that has already printed a full report
// and only needs the process to exit non-zero.
var ErrReported = errors.New("command reported a failure")

// getwd is indirected so tests can point a command at a temporary project
// without changing the working directory of the whole process.
var getwd = os.Getwd

// UsageError marks an error caused by invalid command line usage. The entry
// point maps it to exit code 2, following the Unix convention.
type UsageError struct {
	// Err is the underlying cobra error.
	Err error
}

// Error implements the error interface.
func (e *UsageError) Error() string { return e.Err.Error() }

// Unwrap exposes the underlying error to errors.Is and errors.As.
func (e *UsageError) Unwrap() error { return e.Err }

// ExitCode returns the process exit code for err.
func ExitCode(err error) int {
	var usage *UsageError
	if errors.As(err, &usage) {
		return 2
	}
	return 1
}

// NewRootCommand builds the feather command tree.
//
// version is injected at build time so `feather --version` matches the git tag
// the binary was built from.
func NewRootCommand(version string) *cobra.Command {
	root := &cobra.Command{
		Use:   "feather",
		Short: "Add a feature. Not a headache.",
		Long: `feather scaffolds complete vertical-slice features into an existing Go
project: handler, service, repository, model and tests — wired into the router
and the dependency container with no manual edits.

Create a project once with ` + "`feather init`" + `, then add features to it forever
with ` + "`feather new feature`" + `.`,
		Example: `  feather init demo --module github.com/you/demo
  cd demo
  feather new feature order
  go build ./...`,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.SetVersionTemplate("feather {{.Version}}\n")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &UsageError{Err: err}
	})

	root.AddCommand(
		newInitCommand(),
		newNewCommand(),
		newRemoveCommand(),
		newDoctorCommand(),
	)

	return root
}

// usageArgs classifies a cobra argument validator's error as a usage error.
func usageArgs(validate cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := validate(cmd, args); err != nil {
			return &UsageError{Err: err}
		}
		return nil
	}
}
