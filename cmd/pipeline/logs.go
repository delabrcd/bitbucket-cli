package pipeline

import (
	"fmt"
	"os"

	plcommon "github.com/gildas/bitbucket-cli/cmd/pipeline/common"
	"github.com/gildas/bitbucket-cli/cmd/pipeline/step"
	"github.com/gildas/go-errors"
	"github.com/gildas/go-logger"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:     "logs [flags] [pipeline-build-number-or-uuid]",
	Aliases: []string{"log"},
	Short:   "display a pipeline's step logs (latest pipeline's failed step by default)",
	Long: `display a pipeline's step logs.

This is a shortcut for "bb pipeline step logs". With no build number the latest
pipeline is used; with no step selector the failed step(s) are shown (or the
sole step). Use --step to pick one, --failed for only failed steps, or --all
for every step.`,
	Args:              cobra.RangeArgs(0, 1),
	ValidArgsFunction: logsValidArgs,
	RunE:              logsProcess,
}

var logsOptions struct {
	Step   string
	Failed bool
	All    bool
}

func init() {
	Command.AddCommand(logsCmd)

	logsCmd.Flags().StringVar(&logsOptions.Step, "step", "", "Step to show (uuid or name)")
	logsCmd.Flags().BoolVar(&logsOptions.Failed, "failed", false, "Show the logs of the pipeline's failed step(s)")
	logsCmd.Flags().BoolVar(&logsOptions.All, "all", false, "Show the logs of every step")
	logsCmd.MarkFlagsMutuallyExclusive("step", "failed", "all")
}

func logsValidArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	ids, err := plcommon.GetPipelineIDs(cmd.Context(), cmd, args, toComplete)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return []string{}, cobra.ShellCompDirectiveError
	}
	return ids, cobra.ShellCompDirectiveNoFileComp
}

func logsProcess(cmd *cobra.Command, args []string) error {
	log := logger.Must(logger.FromContext(cmd.Context())).Child(cmd.Parent().Name(), "logs")

	var pipelineID string
	if len(args) == 1 {
		pipelineID = args[0]
	} else {
		latest, err := plcommon.GetLatestPipelineID(log.ToContext(cmd.Context()), cmd)
		if err != nil {
			return errors.Join(errors.New("failed to resolve the latest pipeline"), err)
		}
		pipelineID = latest
		fmt.Fprintf(os.Stderr, "Using latest pipeline #%s\n", pipelineID)
	}

	opts := step.LogOptions{Step: logsOptions.Step, Failed: logsOptions.Failed, All: logsOptions.All}
	return step.SelectAndDumpLogs(log.ToContext(cmd.Context()), cmd, pipelineID, opts, os.Stdout)
}
