package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cybersafetyid/featherctl/internal/fsutil"
	"github.com/cybersafetyid/featherctl/internal/generator"
)

// newInitCommand builds `feather init`.
func newInitCommand() *cobra.Command {
	var (
		module   string
		force    bool
		dryRun   bool
		validate bool
	)

	cmd := &cobra.Command{
		Use:   "init <project-name>",
		Short: "Scaffold a new vertical-slice Go project",
		Long: `init creates a Go project laid out for vertical slice architecture: one
package per feature, a router and a dependency container that
` + "`feather new feature`" + ` can wire into, plus tests, lint configuration and a
Makefile.

The generated project has no third-party dependencies.`,
		Example: `  feather init demo --module github.com/you/demo
  feather init demo --module github.com/you/demo --dry-run`,
		Args: usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			dir, err := resolveTarget(name)
			if err != nil {
				return err
			}

			modulePath, err := resolveModule(cmd, module)
			if err != nil {
				return err
			}

			overwrite := force
			if !overwrite && !dryRun {
				confirmed, err := confirmOverwrite(cmd, dir, name)
				if err != nil {
					return err
				}
				overwrite = confirmed
			}

			report, err := generator.InitProject(generator.ProjectOptions{
				Dir:    dir,
				Name:   name,
				Module: modulePath,
				Force:  overwrite,
				DryRun: dryRun,
			})
			if err != nil {
				return err
			}

			out := newPrinter(cmd.OutOrStdout(), cmd.ErrOrStderr())
			out.report(report)

			if dryRun {
				out.dryRun()
				return nil
			}

			target := targetPath(dir)
			out.summary("Project %q is ready. Next:", name)
			out.hint("cd %s", target)
			out.hint("feather new feature order")
			out.hint("go build ./...")

			return validateProject(cmd, out, dir, validate, dryRun)
		},
	}

	cmd.Flags().StringVar(&module, "module", "", "Go module path of the new project, for example github.com/you/demo")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing files without asking")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be created without writing anything")
	cmd.Flags().BoolVar(&validate, "validate", false, "compile the new project once it has been created")

	return cmd
}

// resolveModule returns the module path from the flag, or asks for it when the
// command runs interactively. feather never invents an import path.
func resolveModule(cmd *cobra.Command, flagValue string) (string, error) {
	if value := strings.TrimSpace(flagValue); value != "" {
		return value, nil
	}

	if !isTerminalReader(cmd.InOrStdin()) {
		return "", errors.New("a Go module path is required: pass --module github.com/you/myapp, " +
			"because feather never guesses an import path for you")
	}

	answer, err := promptLine(cmd, "Go module path (for example github.com/you/myapp): ")
	if err != nil {
		return "", err
	}
	if answer == "" {
		return "", errors.New("a Go module path is required")
	}

	return answer, nil
}

// confirmOverwrite asks before writing into a directory that already has
// content. It returns false when the generator should report the conflict.
func confirmOverwrite(cmd *cobra.Command, dir, name string) (bool, error) {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false, nil
	}

	nonEmpty, err := fsutil.DirHasEntries(dir)
	if err != nil || !nonEmpty {
		return false, nil
	}

	if !isTerminalReader(cmd.InOrStdin()) {
		return false, nil
	}

	answer, err := promptLine(cmd, fmt.Sprintf("%s already exists and is not empty. Overwrite it? [y/N] ", name))
	if err != nil {
		return false, err
	}

	switch strings.ToLower(answer) {
	case "y", "yes":
		return true, nil
	default:
		return false, errors.New("aborted: the directory was left untouched")
	}
}

// promptLine writes question and reads one line of input.
func promptLine(cmd *cobra.Command, question string) (string, error) {
	fmt.Fprint(cmd.OutOrStdout(), question)

	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// validateProject optionally compiles the freshly generated project.
func validateProject(cmd *cobra.Command, out *printer, dir string, validate, dryRun bool) error {
	if !validate || dryRun {
		return nil
	}

	if _, err := generator.Build(cmd.Context(), dir); err != nil {
		return fmt.Errorf("the generated project does not compile: %w", err)
	}

	out.line(out.styles.success, symbolOK, "go build ./... succeeded")
	return nil
}

// resolveTarget turns the project argument into an absolute directory, resolved
// against the working directory.
func resolveTarget(name string) (string, error) {
	if filepath.IsAbs(name) {
		return filepath.Clean(name), nil
	}

	wd, err := getwd()
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", name, err)
	}

	return filepath.Join(wd, name), nil
}

// targetPath renders dir the way the user is most likely to type it.
func targetPath(dir string) string {
	wd, err := getwd()
	if err != nil {
		return filepath.ToSlash(dir)
	}

	rel, err := filepath.Rel(wd, dir)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(dir)
	}
	return filepath.ToSlash(rel)
}
