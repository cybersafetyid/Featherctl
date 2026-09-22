package generator

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Build runs `go build ./...` in dir and returns the combined output.
//
// It backs the --validate flag and the build check in `feather doctor`. The
// error is returned as-is so callers can tell a compile failure from a missing
// Go toolchain.
func Build(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "go", "build", "./...")
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))

	if err != nil {
		return output, fmt.Errorf("go build ./...: %w", err)
	}
	return output, nil
}
