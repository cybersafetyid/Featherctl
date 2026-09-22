// Command feather-release automates featherctl's releases.
//
// It is maintainer tooling: it never ships in the released `feather` binary
// (GoReleaser builds only ./cmd/feather), but it lives in the repository so the
// release process is code, reviewed and tested like everything else.
//
//	feather-release bump 0.2.0        # changelog + commit + tag v0.2.0
//	feather-release notes v0.2.0      # the release notes for a version
//	feather-release verify v0.2.0     # fail if the changelog lacks that version
//	feather-release check             # fail if any tag lacks a changelog section
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cybersafetyid/featherctl/internal/release"
	"github.com/spf13/cobra"
)

func main() {
	os.Exit(run())
}

// run executes the command tree and returns the process exit code. It exists so
// main can exit without skipping deferred cleanups.
func run() int {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := newRootCommand().ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "feather-release: %v\n", err)
		return exitCode(err)
	}
	return 0
}

// usageError marks invalid command line usage, which exits with status 2.
type usageError struct{ err error }

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

// exitCode maps an error to a process exit code: 2 for a usage error, 1 for
// everything that failed while doing the work.
func exitCode(err error) int {
	var usage *usageError
	if errors.As(err, &usage) {
		return 2
	}
	return 1
}

// exactlyOne validates that a command received a single argument.
func exactlyOne(what string) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) != 1 {
			return &usageError{err: fmt.Errorf("expected one %s, got %d", what, len(args))}
		}
		return nil
	}
}

// noArgs validates that a command received no arguments.
func noArgs(_ *cobra.Command, args []string) error {
	if len(args) != 0 {
		return &usageError{err: fmt.Errorf("expected no arguments, got %d", len(args))}
	}
	return nil
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "feather-release",
		Short: "Release tooling for featherctl",
		Long: `feather-release keeps CHANGELOG.md and the git tags in step.

Run it with no arguments for the usage, or read docs/releasing.md.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &usageError{err: err}
	})

	root.AddCommand(
		newBumpCommand(),
		newNotesCommand(),
		newVerifyCommand(),
		newCheckCommand(),
	)

	return root
}

// bumpFlags are the flags shared by `bump`.
type bumpFlags struct {
	dir      string
	date     string
	repoURL  string
	dryRun   bool
	noCommit bool
	noTag    bool
	push     bool
	force    bool
}

func newBumpCommand() *cobra.Command {
	flags := &bumpFlags{}

	cmd := &cobra.Command{
		Use:   "bump <version>",
		Short: "Record a version in CHANGELOG.md, commit it and tag it",
		Long: `bump promotes the [Unreleased] section of CHANGELOG.md into a dated
section for <version>, re-points the comparison links at the new tag, commits
the changelog and creates the annotated tag. Pushing the tag triggers the
release workflow, which refuses to publish a version the changelog does not
document.

If CHANGELOG.md already has a section for <version>, bump keeps it as it is and
goes straight to the commit and the tag.`,
		Example: `  feather-release bump 0.2.0
  feather-release bump v0.2.0 --dry-run
  feather-release bump 0.2.0 --push`,
		Args: exactlyOne("version"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.push && (flags.dryRun || flags.noTag) {
				return &usageError{err: errors.New("--push needs a tag: it cannot be combined with --dry-run or --no-tag")}
			}

			version, err := release.NormalizeVersion(args[0])
			if err != nil {
				return &usageError{err: err}
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Releasing featherctl v%s\n", version)

			result, err := release.Bump(cmd.Context(), release.Options{
				Dir:      flags.dir,
				Version:  version,
				Date:     flags.date,
				RepoURL:  flags.repoURL,
				DryRun:   flags.dryRun,
				NoCommit: flags.noCommit,
				NoTag:    flags.noTag,
				Push:     flags.push,
				Force:    flags.force,
				Out:      out,
			})
			if err != nil {
				return err
			}

			if flags.dryRun {
				fmt.Fprintf(out, "\nDry run: nothing was written, committed or tagged.\n")
				return nil
			}

			fmt.Fprintf(out, "\nv%s is ready.\n", result.Version)
			if !result.Pushed && !flags.noCommit {
				fmt.Fprintf(out, "Push it to publish: git push origin HEAD --follow-tags\n")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flags.dir, "dir", "", "repository directory (default: the current one)")
	cmd.Flags().StringVar(&flags.date, "date", "", "date to stamp on the new section (default: today, UTC)")
	cmd.Flags().StringVar(&flags.repoURL, "repo-url", "", "repository URL for the comparison links (default: the origin remote)")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false, "report what would change without touching anything")
	cmd.Flags().BoolVar(&flags.noCommit, "no-commit", false, "update CHANGELOG.md without committing it")
	cmd.Flags().BoolVar(&flags.noTag, "no-tag", false, "commit the changelog without tagging")
	cmd.Flags().BoolVar(&flags.push, "push", false, "push the commit and the tag to origin")
	cmd.Flags().BoolVar(&flags.force, "force", false, "bump even when CHANGELOG.md has uncommitted changes")

	return cmd
}

func newNotesCommand() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "notes <version>",
		Short: "Print the CHANGELOG.md section for a version",
		Long: `notes prints the changelog section for <version>, which is what a release
page should show. It is the body only: the title of the release already names
the version.`,
		Args: exactlyOne("version"),
		RunE: func(cmd *cobra.Command, args []string) error {
			notes, err := release.Notes(cmd.Context(), dir, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), notes)
			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "repository directory (default: the current one)")
	return cmd
}

func newVerifyCommand() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "verify <version>",
		Short: "Fail unless CHANGELOG.md documents a version",
		Long: `verify is the gate the release workflow runs before publishing: it fails
with a clear message when the tag has no changelog section, so no release page
is ever published with empty notes.`,
		Args: exactlyOne("version"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := release.Verify(cmd.Context(), dir, args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s documents %s\n", release.ChangelogFile, strings.TrimPrefix(args[0], "v"))
			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "repository directory (default: the current one)")
	return cmd
}

func newCheckCommand() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Fail if any tag has no CHANGELOG.md section",
		Long: `check walks every v* tag in the repository and reports the ones that have
no matching changelog section. Run it in CI to catch a release that was tagged
by hand.`,
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			missing, err := release.CheckTags(cmd.Context(), dir)
			if err != nil {
				return err
			}
			if len(missing) > 0 {
				return fmt.Errorf("%s has no section for: %s", release.ChangelogFile, strings.Join(missing, ", "))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "every tag is documented in %s\n", release.ChangelogFile)
			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "repository directory (default: the current one)")
	return cmd
}
