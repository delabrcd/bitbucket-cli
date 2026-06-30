package runner

import (
	"github.com/gildas/bitbucket-cli/cmd/common"
	"github.com/gildas/bitbucket-cli/cmd/profile"
	"github.com/gildas/go-core"
	"github.com/gildas/go-errors"
	"github.com/gildas/go-flags"
	"github.com/gildas/go-logger"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "list the runners of the current repository (or workspace with --workspace-level)",
	Args:    cobra.NoArgs,
	RunE:    listProcess,
}

var listOptions struct {
	Columns    *flags.EnumSliceFlag
	SortBy     *flags.EnumFlag
	PageLength int
	Limit      int
}

func init() {
	Command.AddCommand(listCmd)

	listOptions.Columns = flags.NewEnumSliceFlagWithAllAllowed(columns.Columns()...)
	listOptions.SortBy = flags.NewEnumFlag(columns.Sorters()...)
	listCmd.Flags().Var(listOptions.Columns, "columns", "Comma-separated list of columns to display")
	listCmd.Flags().Var(listOptions.SortBy, "sort", "Column to sort by")
	listCmd.Flags().IntVar(&listOptions.PageLength, "page-length", 0, "Number of items per page to retrieve from Bitbucket. Default is the profile's default page length")
	listCmd.Flags().IntVar(&listOptions.Limit, "limit", 0, "Maximum total number of items to retrieve. 0 means no limit")
	_ = listCmd.RegisterFlagCompletionFunc(listOptions.Columns.CompletionFunc("columns"))
	_ = listCmd.RegisterFlagCompletionFunc(listOptions.SortBy.CompletionFunc("sort"))
}

func listProcess(cmd *cobra.Command, args []string) error {
	log := logger.Must(logger.FromContext(cmd.Context())).Child(cmd.Parent().Name(), "list")

	log.Infof("Listing runners")
	if !common.WhatIf(log.ToContext(cmd.Context()), cmd, "Showing runners") {
		return nil
	}

	path, err := runnerBasePath(cmd.Context(), cmd)
	if err != nil {
		return errors.Join(errors.New("failed to resolve runner base path"), err)
	}

	runners, err := profile.GetAll[Runner](log.ToContext(cmd.Context()), cmd, path)
	if err != nil {
		return errors.Join(errors.New("failed to retrieve runners"), err)
	}
	if len(runners) == 0 {
		log.Infof("No runner found")
		return nil
	}
	log.Debugf("Found %d runner(s)", len(runners))
	core.Sort(runners, columns.SortBy(listOptions.SortBy.Value))
	return profile.Current.Print(cmd.Context(), cmd, Runners(runners))
}
