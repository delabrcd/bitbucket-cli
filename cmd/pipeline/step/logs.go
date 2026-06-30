package step

import (
	"os"

	"github.com/gildas/bitbucket-cli/cmd/common"
	plcommon "github.com/gildas/bitbucket-cli/cmd/pipeline/common"
	"github.com/gildas/go-errors"
	"github.com/gildas/go-flags"
	"github.com/gildas/go-logger"
	"github.com/spf13/cobra"
)

var logCmd = &cobra.Command{
	Use:     "logs [flags] [pipeline-step-uuid-or-name]",
	Aliases: []string{"log"},
	Short:   "display the logs of a pipeline step (its failed step by default)",
	Long: `display the logs of a pipeline step.

Name the step with a positional argument or --step. With no step given, the
single step is shown, or the failed step(s) when there are several. Use
--failed to force only failed steps, or --all to show every step.`,
	Args:              cobra.RangeArgs(0, 1),
	ValidArgsFunction: logValidArgs,
	RunE:              logProcess,
}

var logOptions struct {
	PipelineID *flags.EnumFlag
	Step       string
	Failed     bool
	All        bool
}

func init() {
	Command.AddCommand(logCmd)

	logOptions.PipelineID = flags.NewEnumFlagWithFunc(logCmd, "", plcommon.GetPipelineIDs)
	logCmd.Flags().Var(logOptions.PipelineID, "pipeline", "Pipeline to show step logs from")
	logCmd.Flags().StringVar(&logOptions.Step, "step", "", "Step to show (uuid or name); alternative to the positional argument")
	logCmd.Flags().BoolVar(&logOptions.Failed, "failed", false, "Show the logs of the pipeline's failed step(s)")
	logCmd.Flags().BoolVar(&logOptions.All, "all", false, "Show the logs of every step")
	_ = logCmd.MarkFlagRequired("pipeline")
	logCmd.MarkFlagsMutuallyExclusive("step", "failed", "all")
	_ = logCmd.RegisterFlagCompletionFunc(logOptions.PipelineID.CompletionFunc("pipeline"))
}

func logValidArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	stepIDs, err := GetPipelineStepIDs(cmd.Context(), cmd, logOptions.PipelineID.Value)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return []string{}, cobra.ShellCompDirectiveError
	}
	return common.FilterValidArgs(stepIDs, args, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func logProcess(cmd *cobra.Command, args []string) error {
	log := logger.Must(logger.FromContext(cmd.Context())).Child(cmd.Parent().Name(), "getlogs")

	stepArg := logOptions.Step
	if len(args) == 1 {
		if len(stepArg) > 0 {
			return errors.New("specify the step as a positional argument or with --step, not both")
		}
		stepArg = args[0]
	}

	opts := LogOptions{Step: stepArg, Failed: logOptions.Failed, All: logOptions.All}
	return SelectAndDumpLogs(log.ToContext(cmd.Context()), cmd, logOptions.PipelineID.Value, opts, os.Stdout)
}
