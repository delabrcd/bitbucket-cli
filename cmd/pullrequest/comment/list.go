package comment

import (
	"context"
	"fmt"
	"net/url"
	"slices"

	"github.com/delabrcd/bitbucket-cli/cmd/common"
	"github.com/delabrcd/bitbucket-cli/cmd/profile"
	prcommon "github.com/delabrcd/bitbucket-cli/cmd/pullrequest/common"
	"github.com/delabrcd/bitbucket-cli/cmd/repository"
	"github.com/gildas/go-core"
	"github.com/gildas/go-flags"
	"github.com/gildas/go-logger"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list [flags] [<pullrequest-id>]",
	Short: "list all pullrequest comments",
	Long: `list all pullrequest comments.

Bitbucket reports "resolution" as null on this endpoint, even for a resolved
comment. The resolution column and the --resolved/--unresolved filters therefore
fetch each comment individually, at one request per comment.`,
	Args: prcommon.PullRequestArgs(0),
	RunE: listProcess,
}

var listOptions struct {
	Query         string
	PullRequestID *flags.EnumFlag
	Columns       *flags.EnumSliceFlag
	SortBy        *flags.EnumFlag
	PageLength    int
	Resolved      bool
	Unresolved    bool
}

func init() {
	Command.AddCommand(listCmd)

	listOptions.PullRequestID = flags.NewEnumFlagWithFunc(listCmd, "", prcommon.GetPullRequestIDs)
	listOptions.Columns = flags.NewEnumSliceFlagWithAllAllowed(columns.Columns()...)
	listOptions.SortBy = flags.NewEnumFlag(columns.Sorters()...)
	listCmd.Flags().Var(listOptions.PullRequestID, "pullrequest", "pullrequest to list comments from")
	listCmd.Flags().StringVar(&listOptions.Query, "query", "", "Query string to filter comments")
	listCmd.Flags().Var(listOptions.Columns, "columns", "Comma-separated list of columns to display")
	listCmd.Flags().Var(listOptions.SortBy, "sort", "Column to sort by")
	listCmd.Flags().IntVar(&listOptions.PageLength, "page-length", 0, "Number of items per page to retrieve from Bitbucket. Default is the profile's default page length")
	listCmd.Flags().BoolVar(&listOptions.Resolved, "resolved", false, "Only show resolved comments. Costs one extra request per comment")
	listCmd.Flags().BoolVar(&listOptions.Unresolved, "unresolved", false, "Only show unresolved comments. Costs one extra request per comment")
	listCmd.MarkFlagsMutuallyExclusive("resolved", "unresolved")
	_ = listCmd.RegisterFlagCompletionFunc(listOptions.PullRequestID.CompletionFunc("pullrequest"))
	_ = listCmd.RegisterFlagCompletionFunc(listOptions.Columns.CompletionFunc("columns"))
	_ = listCmd.RegisterFlagCompletionFunc(listOptions.SortBy.CompletionFunc("sort"))
}

func listProcess(cmd *cobra.Command, args []string) (err error) {
	log := logger.Must(logger.FromContext(cmd.Context())).Child(cmd.Parent().Name(), "list")

	args, err = prcommon.TakePullRequestID(cmd, listOptions.PullRequestID, args)
	if err != nil {
		return err
	}

	repository, err := repository.GetRepository(cmd.Context(), cmd)
	if err != nil {
		return err
	}

	uripath := repository.GetPath(fmt.Sprintf("pullrequests/%s/comments", listOptions.PullRequestID.Value))

	if len(listOptions.Query) > 0 {
		uripath = fmt.Sprintf("%s?q=%s", uripath, url.QueryEscape(listOptions.Query))
	}

	log.Infof("Listing all comments from repository %s", repository)
	if !common.WhatIf(log.ToContext(cmd.Context()), cmd, fmt.Sprintf("Showing comments for pullrequest %s in repository %s with profile %s", listOptions.PullRequestID.Value, repository, profile.Current)) {
		return nil
	}

	comments, err := profile.GetAll[Comment](cmd.Context(), cmd, uripath)
	if err != nil {
		return err
	}
	if len(comments) == 0 {
		log.Infof("No comment found")
		return nil
	}
	comments = core.Filter(comments, func(comment Comment) bool {
		return len(comment.Content.Raw) > 0
	})

	if resolutionWanted(cmd) {
		if comments, err = withResolutions(log.ToContext(cmd.Context()), cmd, repository, comments); err != nil {
			return err
		}
		switch {
		case listOptions.Resolved:
			comments = core.Filter(comments, func(comment Comment) bool { return comment.Resolution != nil })
		case listOptions.Unresolved:
			comments = core.Filter(comments, func(comment Comment) bool { return comment.Resolution == nil })
		}
		if len(comments) == 0 {
			log.Infof("No comment found")
			return nil
		}
	}

	core.Sort(comments, columns.SortBy(listOptions.SortBy.Value))
	return profile.Current.Print(cmd.Context(), cmd, Comments(comments))
}

// resolutionWanted reports whether the caller asked for something that needs a real
// resolution value, which the list endpoint does not provide.
func resolutionWanted(cmd *cobra.Command) bool {
	if listOptions.Resolved || listOptions.Unresolved {
		return true
	}
	if flag := cmd.Flag("columns"); flag != nil && flag.Changed {
		return slices.Contains(listOptions.Columns.Values, "resolution")
	}
	return false
}

// withResolutions re-fetches each comment individually, since the list endpoint
// reports resolution as null even for a resolved comment.
func withResolutions(ctx context.Context, cmd *cobra.Command, repository *repository.Repository, comments []Comment) ([]Comment, error) {
	log := logger.Must(logger.FromContext(ctx)).Child("comment", "resolutions")

	log.Infof("Fetching the resolution of %d comments individually", len(comments))
	for i := range comments {
		var full Comment

		path := repository.GetPath("pullrequests", listOptions.PullRequestID.Value, "comments", fmt.Sprintf("%d", comments[i].ID))
		if err := profile.Current.Get(ctx, cmd, path, &full); err != nil {
			return nil, err
		}
		comments[i].Resolution = full.Resolution
	}
	return comments, nil
}
