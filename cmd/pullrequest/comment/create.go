package comment

import (
	"fmt"
	"os"

	"github.com/gildas/bitbucket-cli/cmd/common"
	"github.com/gildas/bitbucket-cli/cmd/profile"
	prcommon "github.com/gildas/bitbucket-cli/cmd/pullrequest/common"
	"github.com/gildas/bitbucket-cli/cmd/repository"
	"github.com/gildas/go-errors"
	"github.com/gildas/go-flags"
	"github.com/gildas/go-logger"
	"github.com/spf13/cobra"
)

type CommentCreator struct {
	Content ContentCreator     `json:"content"           mapstructure:"content"`
	Anchor  *common.FileAnchor `json:"inline,omitempty"  mapstructure:"inline"`
	Parent  *ParentReference   `json:"parent,omitempty"  mapstructure:"parent"`
	Pending *bool              `json:"pending,omitempty" mapstructure:"pending"`
}

type ContentCreator struct {
	Raw string `json:"raw" mapstructure:"raw"`
}

var createCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"add", "new"},
	Short:   "create a pullrequest comment",
	Long: `Create a pullrequest comment.

For an inline (file) comment pass --file together with a line anchor. Bitbucket
anchors an inline comment to one side of the diff, and which side you pick must
match the kind of line:

  --to   <line>  Line number in the NEW (post-change) version of the file.
                 Use for ADDED ("+") lines. Also valid for context lines.
  --from <line>  Line number in the OLD (pre-change) version of the file.
                 Use for REMOVED ("-") lines. Also valid for context lines.
  --line <line>  Alias for --to (NEW side); the common added-line case.

Rule of thumb: a line that exists only after the change must be anchored with
--to/--line; a deleted line with --from; an unchanged context line works with
either. File line numbers from a tool reading the new file (e.g. "grep -n" on
the head revision) are NEW-side numbers and belong on --to/--line.`,
	Args: cobra.NoArgs,
	RunE: createProcess,
}

var createOptions struct {
	PullRequestID *flags.EnumFlag
	Comment       string
	CommentFile   string
	File          string
	From          int
	To            int
	ParentID      int64
	Pending       bool
}

func init() {
	Command.AddCommand(createCmd)

	createOptions.PullRequestID = flags.NewEnumFlagWithFunc(createCmd, "", prcommon.GetPullRequestIDs)
	createCmd.Flags().Var(createOptions.PullRequestID, "pullrequest", "Pullrequest to create comments to")
	createCmd.Flags().StringVar(&createOptions.Comment, "comment", "", "Comment of the pullrequest")
	createCmd.Flags().StringVar(&createOptions.CommentFile, "comment-file", "", "Read the comment from a file (use \"-\" to read from standard input)")
	createCmd.Flags().StringVar(&createOptions.File, "file", "", "File to comment on")
	createCmd.Flags().IntVar(&createOptions.To, "line", 0, "Alias for --to (NEW/post-change side); the common added-line case. Cannot be used with --to")
	createCmd.Flags().IntVar(&createOptions.From, "from", 0, "Anchor on the OLD (pre-change) side of the diff; use for removed lines")
	createCmd.Flags().IntVar(&createOptions.To, "to", 0, "Anchor on the NEW (post-change) side of the diff; use for added/context lines. Cannot be used with --line")
	createCmd.Flags().Int64Var(&createOptions.ParentID, "parent", 0, "Parent comment ID to reply to")
	createCmd.Flags().BoolVar(&createOptions.Pending, "pending", false, "Mark the comment as pending")
	createCmd.MarkFlagsMutuallyExclusive("line", "to")
	_ = createCmd.MarkFlagFilename("comment-file")
	createCmd.MarkFlagsMutuallyExclusive("comment", "comment-file")
	createCmd.MarkFlagsOneRequired("comment", "comment-file")
	_ = createCmd.MarkFlagRequired("pullrequest")
	_ = createCmd.RegisterFlagCompletionFunc(createOptions.PullRequestID.CompletionFunc("pullrequest"))
}

func createProcess(cmd *cobra.Command, args []string) (err error) {
	log := logger.Must(logger.FromContext(cmd.Context())).Child(cmd.Parent().Name(), "create")

	profile, err := profile.GetProfileFromCommand(cmd.Context(), cmd)
	if err != nil {
		return err
	}

	repository, err := repository.GetRepository(cmd.Context(), cmd)
	if err != nil {
		return err
	}

	body := createOptions.Comment
	if cmd.Flag("comment-file").Changed {
		data, rerr := common.ReadFileOrStdin(createOptions.CommentFile)
		if rerr != nil {
			return rerr
		}
		body = string(data)
	}
	if len(body) == 0 {
		return errors.ArgumentMissing.With("comment")
	}

	payload := CommentCreator{
		Content: ContentCreator{Raw: body},
	}

	if createOptions.ParentID > 0 {
		payload.Parent = &ParentReference{ID: createOptions.ParentID}
	}

	if createOptions.File != "" {
		payload.Anchor = &common.FileAnchor{
			Path: createOptions.File,
		}
		if createOptions.From > 0 {
			payload.Anchor.From = uint64(createOptions.From)
		}
		if createOptions.To > 0 {
			payload.Anchor.To = uint64(createOptions.To)
		}
	} else if createOptions.From > 0 || createOptions.To > 0 {
		return errors.RuntimeError.With("Cannot specify from/to without a file")
	}
	if cmd.Flag("pending").Changed {
		payload.Pending = &createOptions.Pending
	}

	log.Record("payload", payload).Infof("Creating pullrequest comment")
	if !common.WhatIf(log.ToContext(cmd.Context()), cmd, "Creating comment for pullrequest %s", createOptions.PullRequestID.Value) {
		return nil
	}
	var comment Comment

	err = profile.Post(
		log.ToContext(cmd.Context()),
		cmd,
		repository.GetPath("pullrequests", createOptions.PullRequestID.Value, "comments"),
		payload,
		&comment,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create comment for pullrequest %s: %s\n", createOptions.PullRequestID.Value, err)
		os.Exit(1)
	}
	return profile.Print(cmd.Context(), cmd, comment)
}
