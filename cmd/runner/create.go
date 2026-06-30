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

type runnerCreator struct {
	Name   string   `json:"name"`
	Labels []string `json:"labels"`
}

var createCmd = &cobra.Command{
	Use:   "create [flags]",
	Short: "create a self-hosted runner and print its one-time OAuth credentials",
	Args:  cobra.NoArgs,
	RunE:  createProcess,
}

var createOptions struct {
	Name   string
	Labels []string
}

func init() {
	Command.AddCommand(createCmd)

	createCmd.Flags().StringVar(&createOptions.Name, "name", "", "Name of the runner")
	createCmd.Flags().StringArrayVar(&createOptions.Labels, "label", []string{}, "Runner label (repeatable). Include one OS label (linux, linux.arm64, linux.shell, windows, macos). self.hosted is added automatically if missing.")
	_ = createCmd.MarkFlagRequired("name")
}

func createProcess(cmd *cobra.Command, args []string) error {
	log := logger.Must(logger.FromContext(cmd.Context())).Child(cmd.Parent().Name(), "create")

	currentProfile, err := profile.GetProfileFromCommand(cmd.Context(), cmd)
	if err != nil {
		return err
	}

	labels := createOptions.Labels
	hasSelfHosted := false
	for _, label := range labels {
		if label == "self.hosted" {
			hasSelfHosted = true
			break
		}
	}
	if !hasSelfHosted {
		labels = append([]string{"self.hosted"}, labels...)
	}

	payload := runnerCreator{
		Name:   createOptions.Name,
		Labels: labels,
	}

	path, err := runnerBasePath(cmd.Context(), cmd)
	if err != nil {
		return errors.Join(errors.Errorf("failed to resolve runner base path"), err)
	}

	log.Record("payload", payload).Infof("Creating runner %s", createOptions.Name)
	if !common.WhatIf(log.ToContext(cmd.Context()), cmd, "Creating runner %s", createOptions.Name) {
		return nil
	}

	var created Runner
	err = currentProfile.Post(log.ToContext(cmd.Context()), cmd, path, payload, &created)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create runner %s: %s\n", createOptions.Name, err)
		os.Exit(1)
	}
	return currentProfile.Print(cmd.Context(), cmd, created)
}
