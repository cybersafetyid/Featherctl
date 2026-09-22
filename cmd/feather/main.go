// Command feather scaffolds fully-wired vertical-slice features into an
// existing Go project.
package main

import (
	"errors"
	"os"

	"github.com/cybersafetyid/featherctl/internal/cli"
)

// version is the version reported by `feather --version`. Release builds set it
// with -ldflags "-X main.version=v0.1.0".
var version = "dev"

func main() {
	root := cli.NewRootCommand(version)

	if err := root.Execute(); err != nil {
		// A command that already printed its own report only needs the exit
		// code; printing the error again would duplicate the output.
		if errors.Is(err, cli.ErrReported) {
			os.Exit(cli.ExitCode(err))
		}

		cli.PrintError(os.Stderr, err)
		os.Exit(cli.ExitCode(err))
	}
}
