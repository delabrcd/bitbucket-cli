package runner

import (
	"fmt"
	"os"

	"github.com/delabrcd/bitbucket-cli/cmd/common"
	"github.com/delabrcd/bitbucket-cli/cmd/profile"
	"github.com/gildas/go-errors"
	"github.com/gildas/go-logger"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:               "delete [flags] <runner-uuid...>",
	Aliases:           []string{"remove", "rm"},
	Short:             "delete one or more runners by UUID",
	Args:              cobra.MinimumNArgs(1),
	ValidArgsFunction: deleteValidArgs,
	RunE:              deleteProcess,
}

func init() {
	Command.AddCommand(deleteCmd)
}

func deleteValidArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
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

func deleteProcess(cmd *cobra.Command, args []string) error {
	log := logger.Must(logger.FromContext(cmd.Context())).Child(cmd.Parent().Name(), "delete")

	currentProfile, err := profile.GetProfileFromCommand(cmd.Context(), cmd)
	if err != nil {
		return err
	}

	var merr errors.MultiError
	for _, arg := range args {
		if common.WhatIf(log.ToContext(cmd.Context()), cmd, "Deleting runner %s", arg) {
			uuid := NormalizeRunnerUUID(arg)
			path, err := runnerBasePath(cmd.Context(), cmd, uuid)
			if err != nil {
				if currentProfile.ShouldStopOnError(cmd) {
					return errors.Join(
						errors.Errorf("failed to resolve path for runner: %s", arg),
						err,
					)
				}
				merr.Append(err)
				continue
			}
			err = currentProfile.Delete(log.ToContext(cmd.Context()), cmd, path, nil)
			if err != nil {
				if currentProfile.ShouldStopOnError(cmd) {
					fmt.Fprintf(os.Stderr, "Failed to delete runner %s: %s\n", arg, err)
					os.Exit(1)
				} else {
					merr.Append(err)
				}
			}
			log.Infof("Runner %s deleted", arg)
		}
	}
	if !merr.IsEmpty() && currentProfile.ShouldWarnOnError(cmd) {
		fmt.Fprintf(os.Stderr, "Failed to delete these runners: %s\n", merr)
		return nil
	}
	if currentProfile.ShouldIgnoreErrors(cmd) {
		log.Warnf("Failed to delete these runners, but ignoring errors: %s", merr)
		return nil
	}
	return merr.AsError()
}
