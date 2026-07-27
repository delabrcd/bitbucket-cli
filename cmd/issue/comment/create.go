package comment

import (
	"fmt"
	"os"

	"github.com/delabrcd/bitbucket-cli/cmd/common"
	"github.com/delabrcd/bitbucket-cli/cmd/profile"
	"github.com/delabrcd/bitbucket-cli/cmd/repository"
	"github.com/gildas/go-errors"
	"github.com/gildas/go-flags"
	"github.com/gildas/go-logger"
	"github.com/spf13/cobra"
)

type CommentCreator struct {
	Content common.RenderedText `json:"content" mapstructure:"content"`
}

var createCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"add", "new"},
	Short:   "create an issue comment",
	Args:    cobra.NoArgs,
	RunE:    createProcess,
}

var createOptions struct {
	IssueID     *flags.EnumFlag
	Comment     string
	CommentFile string
}

func init() {
	Command.AddCommand(createCmd)

	createOptions.IssueID = flags.NewEnumFlagWithFunc(createCmd, "", GetIssueIDs)
	createCmd.Flags().Var(createOptions.IssueID, "issue", "Issue to create comments to")
	createCmd.Flags().StringVar(&createOptions.Comment, "comment", "", "Comment of the issue")
	createCmd.Flags().StringVar(&createOptions.CommentFile, "comment-file", "", "Read the comment from a file (use \"-\" to read from standard input)")
	_ = createCmd.MarkFlagFilename("comment-file")
	createCmd.MarkFlagsMutuallyExclusive("comment", "comment-file")
	createCmd.MarkFlagsOneRequired("comment", "comment-file")
	_ = createCmd.MarkFlagRequired("issue")
	_ = createCmd.RegisterFlagCompletionFunc(createOptions.IssueID.CompletionFunc("issue"))
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
		Content: common.RenderedText{
			Raw:    common.MaybeFixupMarkdown(cmd, body),
			Markup: "markdown",
		},
	}

	log.Record("payload", payload).Infof("Creating issue comment")
	if !common.WhatIf(log.ToContext(cmd.Context()), cmd, "Creating comment for issue %s", createOptions.IssueID) {
		return nil
	}
	var comment Comment

	err = profile.Post(
		log.ToContext(cmd.Context()),
		cmd,
		repository.GetPath("issues", createOptions.IssueID.Value, "comments"),
		payload,
		&comment,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create comment for issue %s: %s\n", createOptions.IssueID.Value, err)
		os.Exit(1)
	}
	return profile.Print(cmd.Context(), cmd, comment)
}
