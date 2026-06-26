package common

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/gildas/go-errors"
	"github.com/gildas/go-logger"
)

// GitOutput runs git with the given arguments in the current directory and
// returns its standard output with the trailing newline trimmed.
func GitOutput(ctx context.Context, args ...string) (string, error) {
	log := logger.Must(logger.FromContext(ctx)).Child("git", "exec")
	var stdout, stderr bytes.Buffer
	command := exec.CommandContext(ctx, "git", args...)
	command.Stdout = &stdout
	command.Stderr = &stderr
	log.Debugf("Executing: git %s", strings.Join(args, " "))
	if err := command.Run(); err != nil {
		return "", errors.RuntimeError.Wrap(fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String())))
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}
