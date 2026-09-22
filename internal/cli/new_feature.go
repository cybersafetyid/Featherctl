package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cybersafetyid/featherctl/internal/config"
	"github.com/cybersafetyid/featherctl/internal/generator"
)

// newNewCommand builds the `feather new` group.
func newNewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new",
		Short: "Add something new to an existing project",
		Long:  "new groups the commands that add generated code to a project created by `feather init`.",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newFeatureCommand())

	return cmd
}

// newFeatureCommand builds `feather new feature`.
func newFeatureCommand() *cobra.Command {
	var (
		force    bool
		dryRun   bool
		validate bool
	)

	cmd := &cobra.Command{
		Use:   "feature <name>",
		Short: "Generate a feature slice and wire it into the project",
		Long: `feature generates one vertical slice — model, repository, service, handler and
tests — inside internal/features/<name>, then registers its routes in the
router and its dependencies in the container.

The feature name may be written as order, user-profile or userProfile.`,
		Example: `  feather new feature order
  feather new feature user-profile --dry-run`,
		Args: usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			root, err := findProject()
			if err != nil {
				return err
			}

			report, err := generator.NewFeature(generator.FeatureOptions{
				Root:   root,
				Name:   name,
				Force:  force,
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

			out.summary("Feature %q is ready. Run `go build ./...` to verify.", name)

			if validate {
				if _, err := generator.Build(cmd.Context(), root); err != nil {
					return fmt.Errorf("the generated feature does not compile: %w", err)
				}
				out.line(out.styles.success, symbolOK, "go build ./... succeeded")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite the feature if it already exists")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be generated and wired without writing anything")
	cmd.Flags().BoolVar(&validate, "validate", false, "compile the project after generating")

	return cmd
}

// findProject locates the root of the project the command is running inside.
func findProject() (string, error) {
	wd, err := getwd()
	if err != nil {
		return "", err
	}

	root, err := config.Find(wd)
	if err != nil {
		return "", fmt.Errorf("%w\n  Run this command from inside a project created by `feather init`", err)
	}

	return root, nil
}
