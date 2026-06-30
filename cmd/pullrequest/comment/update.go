package comment

import (
	"fmt"
	"os"

	"github.com/delabrcd/bitbucket-cli/cmd/common"
	"github.com/delabrcd/bitbucket-cli/cmd/profile"
	"github.com/delabrcd/bitbucket-cli/cmd/pullrequest/common"
	"github.com/delabrcd/bitbucket-cli/cmd/repository"
	"github.com/gildas/go-errors"
	"github.com/gildas/go-flags"
	"github.com/gildas/go-logger"
	"github.com/spf13/cobra"
)

type CommentUpdator struct {
	Content ContentUpdator     `json:"content"           mapstructure:"content"`
	Anchor  *common.FileAnchor `json:"inline,omitempty"  mapstructure:"inline"`
	Parent  *ParentReference   `json:"parent,omitempty"  mapstructure:"parent"`
	Pending *bool              `json:"pending,omitempty" mapstructure:"pending"`
}

type ContentUpdator struct {
	Raw string `json:"raw" mapstructure:"raw"`
}

var updateCmd = &cobra.Command{
	Use:     "update [flags] <comment-id>",
	Aliases: []string{"edit"},
	Short:   "update an issue comment by its <comment-id>.",
	Long: `Update a pullrequest comment by its <comment-id>.

For an inline (file) comment the line anchor follows the same rules as
'comment create': Bitbucket anchors to one side of the diff and the side must
match the kind of line:

  --to   <line>  Line number in the NEW (post-change) version of the file.
                 Use for ADDED ("+") lines. Also valid for context lines.
  --from <line>  Line number in the OLD (pre-change) version of the file.
                 Use for REMOVED ("-") lines. Also valid for context lines.
  --line <line>  Alias for --to (NEW side); the common added-line case.

Rule of thumb: a line that exists only after the change must be anchored with
--to/--line; a deleted line with --from; an unchanged context line works with
either.`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: updateValidArgs,
	RunE:              updateProcess,
}

var updateOptions struct {
	PullRequestID *flags.EnumFlag
	Comment       string
	File          string
	From          int
	To            int
	ParentID      int64
	Pending       bool
}

func init() {
	Command.AddCommand(updateCmd)

	updateOptions.PullRequestID = flags.NewEnumFlagWithFunc(updateCmd, "", prcommon.GetPullRequestIDs)
	updateCmd.Flags().Var(updateOptions.PullRequestID, "pullrequest", "Pullrequest to update comments to")
	updateCmd.Flags().StringVar(&updateOptions.Comment, "comment", "", "Updated comment of the pullrequest")
	updateCmd.Flags().StringVar(&updateOptions.File, "file", "", "File to comment on")
	updateCmd.Flags().IntVar(&updateOptions.To, "line", 0, "Alias for --to (NEW/post-change side); the common added-line case. Cannot be used with --to")
	updateCmd.Flags().IntVar(&updateOptions.From, "from", 0, "Anchor on the OLD (pre-change) side of the diff; use for removed lines")
	updateCmd.Flags().IntVar(&updateOptions.To, "to", 0, "Anchor on the NEW (post-change) side of the diff; use for added/context lines. Cannot be used with --line")
	updateCmd.Flags().Int64Var(&updateOptions.ParentID, "parent", 0, "Parent comment ID to reply to")
	updateCmd.Flags().BoolVar(&updateOptions.Pending, "pending", false, "Mark the comment as pending")
	updateCmd.MarkFlagsMutuallyExclusive("line", "to")
	_ = updateCmd.MarkFlagRequired("pullrequest")
	_ = updateCmd.MarkFlagRequired("comment")
	_ = updateCmd.RegisterFlagCompletionFunc(updateOptions.PullRequestID.CompletionFunc("pullrequest"))
}

func updateValidArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	commentIDs, err := GetPullRequestCommentIDs(cmd.Context(), cmd, args, toComplete)
	if err != nil {
		return []string{}, cobra.ShellCompDirectiveNoFileComp
	}
	return commentIDs, cobra.ShellCompDirectiveNoFileComp
}

func updateProcess(cmd *cobra.Command, args []string) (err error) {
	log := logger.Must(logger.FromContext(cmd.Context())).Child(cmd.Parent().Name(), "update")

	profile, err := profile.GetProfileFromCommand(cmd.Context(), cmd)
	if err != nil {
		return err
	}

	repository, err := repository.GetRepository(cmd.Context(), cmd)
	if err != nil {
		return err
	}

	payload := CommentUpdator{
		Content: ContentUpdator{Raw: updateOptions.Comment},
	}

	if updateOptions.File != "" {
		payload.Anchor = &common.FileAnchor{
			Path: updateOptions.File,
		}
		if updateOptions.From > 0 {
			payload.Anchor.From = uint64(updateOptions.From)
		}
		if updateOptions.To > 0 {
			payload.Anchor.To = uint64(updateOptions.To)
		}
	} else if updateOptions.From > 0 || updateOptions.To > 0 {
		return errors.RuntimeError.With("Cannot specify from/to without a file")
	}

	if cmd.Flag("pending").Changed {
		payload.Pending = &updateOptions.Pending
	}

	if updateOptions.ParentID > 0 {
		payload.Parent = &ParentReference{ID: updateOptions.ParentID}
	}

	log.Record("payload", payload).Infof("Updating pullrequest comment")
	if !common.WhatIf(log.ToContext(cmd.Context()), cmd, "Updating comment %s for pullrequest %s", args[0], updateOptions.PullRequestID.Value) {
		return nil
	}
	var comment Comment

	err = profile.Put(
		log.ToContext(cmd.Context()),
		cmd,
		repository.GetPath("pullrequests", updateOptions.PullRequestID.Value, "comments", args[0]),
		payload,
		&comment,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to update comment for pullrequest %s: %s\n", updateOptions.PullRequestID.Value, err)
		os.Exit(1)
	}
	return profile.Print(cmd.Context(), cmd, comment)
}
