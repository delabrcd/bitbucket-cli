package comment

import (
	"context"
	"fmt"
	"os"

	"github.com/delabrcd/bitbucket-cli/cmd/common"
	"github.com/delabrcd/bitbucket-cli/cmd/profile"
	"github.com/delabrcd/bitbucket-cli/cmd/pullrequest/common"
	"github.com/delabrcd/bitbucket-cli/cmd/repository"
	"github.com/gildas/go-flags"
	"github.com/gildas/go-logger"
	"github.com/spf13/cobra"
)

var resolveCmd = &cobra.Command{
	Use:               "resolve [flags] [<pullrequest-id>] <comment-id>",
	Aliases:           []string{"done"},
	Short:             "resolve a pullrequest comment by its <comment-id>.",
	Args:              prcommon.PullRequestArgs(1),
	ValidArgsFunction: resolveValidArgs,
	RunE:              resolveProcess,
}

var resolveOptions struct {
	PullRequestID *flags.EnumFlag
}

func init() {
	Command.AddCommand(resolveCmd)

	resolveOptions.PullRequestID = flags.NewEnumFlagWithFunc(resolveCmd, "", prcommon.GetPullRequestIDs)
	resolveCmd.Flags().Var(resolveOptions.PullRequestID, "pullrequest", "Pullrequest to resolve comments from")
	_ = resolveCmd.RegisterFlagCompletionFunc(resolveOptions.PullRequestID.CompletionFunc("pullrequest"))
}

func resolveValidArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	commentIDs, err := GetPullRequestCommentIDs(cmd.Context(), cmd, args, toComplete)
	if err != nil {
		return []string{}, cobra.ShellCompDirectiveNoFileComp
	}
	return commentIDs, cobra.ShellCompDirectiveNoFileComp
}

func resolveProcess(cmd *cobra.Command, args []string) (err error) {
	log := logger.Must(logger.FromContext(cmd.Context())).Child(cmd.Parent().Name(), "resolve")

	args, err = prcommon.TakePullRequestID(cmd, resolveOptions.PullRequestID, args)
	if err != nil {
		return err
	}

	profile, err := profile.GetProfileFromCommand(cmd.Context(), cmd)
	if err != nil {
		return err
	}

	repository, err := repository.GetRepository(cmd.Context(), cmd)
	if err != nil {
		return err
	}

	if !common.WhatIf(log.ToContext(cmd.Context()), cmd, "Resolving comment %s from pullrequest %s", args[0], resolveOptions.PullRequestID.Value) {
		return nil
	}

	err = profile.Post(
		log.ToContext(cmd.Context()),
		cmd,
		repository.GetPath("pullrequests", resolveOptions.PullRequestID.Value, "comments", args[0], "resolve"),
		nil,
		nil,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to resolve pullrequest comment %s: %s\n", args[0], err)
		if parentID, ok := parentCommentID(log.ToContext(cmd.Context()), cmd, profile, repository, resolveOptions.PullRequestID.Value, args[0]); ok {
			fmt.Fprintf(os.Stderr, "\nComment %s is a reply to %d. Resolve the thread's first comment instead:\n  bb pr comment resolve %s %d\n", args[0], parentID, resolveOptions.PullRequestID.Value, parentID)
		}
		os.Exit(1)
	}
	log.Infof("Pullrequest comment %s resolved", args[0])
	return nil
}

// parentCommentID reports the ID of the comment this one replies to, if any.
func parentCommentID(ctx context.Context, cmd *cobra.Command, profile *profile.Profile, repository *repository.Repository, pullRequestID, commentID string) (parentID int, ok bool) {
	log := logger.Must(logger.FromContext(ctx)).Child("comment", "parent")

	var comment Comment

	if err := profile.Get(ctx, cmd, repository.GetPath("pullrequests", pullRequestID, "comments", commentID), &comment); err != nil {
		log.Debugf("Could not fetch comment %s to look for a parent: %s", commentID, err)
		return 0, false
	}
	if comment.Parent == nil || comment.Parent.ID == 0 {
		return 0, false
	}
	return comment.Parent.ID, true
}
