package runner

import (
	"github.com/gildas/bitbucket-cli/cmd/common"
	"github.com/gildas/bitbucket-cli/cmd/profile"
	"github.com/gildas/go-errors"
	"github.com/gildas/go-flags"
	"github.com/gildas/go-logger"
	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:               "get [flags] <runner-uuid>",
	Aliases:           []string{"show", "info", "display"},
	Short:             "get a runner by its UUID",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: getValidArgs,
	RunE:              getProcess,
}

var getOptions struct {
	Columns *flags.EnumSliceFlag
}

func init() {
	Command.AddCommand(getCmd)

	getOptions.Columns = flags.NewEnumSliceFlag(columns.Columns()...)
	getCmd.Flags().Var(getOptions.Columns, "columns", "Comma-separated list of columns to display")
	_ = getCmd.RegisterFlagCompletionFunc(getOptions.Columns.CompletionFunc("columns"))
}

func getValidArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	uuids, err := GetRunnerUUIDs(cmd.Context(), cmd)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return []string{}, cobra.ShellCompDirectiveError
	}
	return common.FilterValidArgs(uuids, args, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func getProcess(cmd *cobra.Command, args []string) error {
	log := logger.Must(logger.FromContext(cmd.Context())).Child(cmd.Parent().Name(), "get")

	currentProfile, err := profile.GetProfileFromCommand(cmd.Context(), cmd)
	if err != nil {
		return err
	}

	uuid := NormalizeRunnerUUID(args[0])
	path, err := runnerBasePath(cmd.Context(), cmd, uuid)
	if err != nil {
		return errors.Join(errors.Errorf("failed to resolve path for runner %s", args[0]), err)
	}

	log.Infof("Displaying runner %s", args[0])
	if !common.WhatIf(log.ToContext(cmd.Context()), cmd, "Showing runner %s", args[0]) {
		return nil
	}

	var runner Runner
	err = currentProfile.Get(log.ToContext(cmd.Context()), cmd, path, &runner)
	if err != nil {
		return errors.Join(errors.Errorf("failed to get runner %s", args[0]), err)
	}
	return currentProfile.Print(cmd.Context(), cmd, runner)
}
