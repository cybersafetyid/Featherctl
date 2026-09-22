package cli

import (
	"github.com/spf13/cobra"

	"github.com/cybersafetyid/featherctl/internal/generator"
)

// newRemoveCommand builds the `feather remove` group.
func newRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove generated code from a project",
		Long:  "remove groups the commands that undo a generation made by `feather new`.",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newRemoveFeatureCommand())

	return cmd
}

// newRemoveFeatureCommand builds `feather remove feature`.
func newRemoveFeatureCommand() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "feature <name>",
		Short: "Delete a feature slice and un-wire it from the project",
		Long: `feature deletes internal/features/<name> and takes the lines featherctl
injected for it out of the router and the container.

If those lines cannot be identified with certainty the command does nothing and
tells you which file to clean up by hand, rather than guessing.`,
		Example: `  feather remove feature order
  feather remove feature order --dry-run`,
		Args: usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			root, err := findProject()
			if err != nil {
				return err
			}

			report, err := generator.RemoveFeature(generator.RemoveFeatureOptions{
				Root:   root,
				Name:   name,
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

			out.summary("Feature %q has been removed. Run `go build ./...` to verify.", name)
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be removed without writing anything")

	return cmd
}
