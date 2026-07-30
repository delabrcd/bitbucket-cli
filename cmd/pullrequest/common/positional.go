package prcommon

import (
	"fmt"

	"github.com/gildas/go-errors"
	"github.com/gildas/go-flags"
	"github.com/spf13/cobra"
)

// PullRequestArgs requires n arguments of the subcommand's own, plus a leading
// pullrequest id when --pullrequest is absent.
func PullRequestArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		want := wanted(cmd, n)
		if len(args) != want {
			return fmt.Errorf("accepts %d arg(s), received %d", want, len(args))
		}
		return nil
	}
}

// PullRequestArgsMinimum is PullRequestArgs for a subcommand taking a variable
// number of its own arguments.
func PullRequestArgsMinimum(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		want := wanted(cmd, n)
		if len(args) < want {
			return fmt.Errorf("requires at least %d arg(s), only received %d", want, len(args))
		}
		return nil
	}
}

func wanted(cmd *cobra.Command, n int) int {
	if flag := cmd.Flag("pullrequest"); flag == nil || !flag.Changed {
		return n + 1
	}
	return n
}

// TakePullRequestID accepts the pullrequest id positionally as well as via
// --pullrequest, and returns the arguments that remain. With --pullrequest, every
// positional belongs to the subcommand; without it, the first positional is the
// pullrequest.
func TakePullRequestID(cmd *cobra.Command, pullRequestID *flags.EnumFlag, args []string) (rest []string, err error) {
	if flag := cmd.Flag("pullrequest"); flag != nil && flag.Changed {
		return args, nil
	}
	if len(args) == 0 {
		return nil, errors.ArgumentMissing.With("pullrequest")
	}

	// Assigned rather than Set: validating against the completion list would cost a
	// round trip listing every pullrequest.
	pullRequestID.Value = args[0]

	return args[1:], nil
}
