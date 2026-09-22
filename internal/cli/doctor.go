package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cybersafetyid/featherctl/internal/generator"
)

// newDoctorCommand builds `feather doctor`.
func newDoctorCommand() *cobra.Command {
	var skipBuild bool

	cmd := &cobra.Command{
		Use:   "doctor [path]",
		Short: "Check that a project is still ready for generation",
		Long: `doctor verifies that feather.yaml parses, that the marker comments
auto-wiring depends on are still present, and that the project compiles.

It exits non-zero when a check fails, so it can be used as a CI gate.`,
		Example: `  feather doctor
  feather doctor ./services/api --skip-build`,
		Args: usageArgs(cobra.MaximumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}

			checks := generator.Doctor(cmd.Context(), generator.DoctorOptions{
				Dir:       dir,
				SkipBuild: skipBuild,
			})

			out := newPrinter(cmd.OutOrStdout(), cmd.ErrOrStderr())
			for _, check := range checks {
				out.check(check)
			}

			if !generator.Failed(checks) {
				out.summary("Project looks healthy.")
				return nil
			}

			out.error(fmt.Errorf("project is not healthy, see the checks above"))
			return ErrReported
		},
	}

	cmd.Flags().BoolVar(&skipBuild, "skip-build", false, "skip compiling the project")

	return cmd
}
