package skill

import (
	"fmt"
	"os"

	"github.com/delabrcd/bitbucket-cli/cmd/common"
	"github.com/gildas/go-errors"
	"github.com/gildas/go-logger"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:     "update",
	Aliases: []string{"sync"},
	Short:   "rewrite out-of-date tracked bb skill installations",
	Long: `update rewrites every tracked skill installation whose contents no longer
match the skill bundled with this build of bb.

Tracked installations that no longer exist on disk are skipped (they may have
been intentionally removed); use "bb skill forget" to stop tracking them, or
"bb skill install" to reinstall.`,
	Args: cobra.NoArgs,
	RunE: updateProcess,
}

var forgetCmd = &cobra.Command{
	Use:   "forget <path>",
	Short: "stop tracking a skill installation path",
	Long: `forget removes a path from the skill registry without touching the file on
disk. Use it to stop bb from reporting or auto-updating an installation you
manage yourself.`,
	Args: cobra.ExactArgs(1),
	RunE: forgetProcess,
}

func init() {
	Command.AddCommand(updateCmd)
	Command.AddCommand(forgetCmd)
}

func updateProcess(cmd *cobra.Command, args []string) error {
	log := logger.Must(logger.FromContext(cmd.Context())).Child(cmd.Parent().Name(), "update")

	state, err := LoadState()
	if err != nil {
		return err
	}

	data, err := assets.ReadFile(skillAsset)
	if err != nil {
		return errors.Join(errors.New("failed to read the bundled skill"), err)
	}

	out := cmd.OutOrStdout()
	updated := 0
	driftedCount := 0
	skippedMissing := 0

	for i := range state.Installations {
		inst := &state.Installations[i]
		drifted, exists, err := IsDrifted(inst.Path)
		if err != nil {
			log.Warnf("failed to check %s: %s", inst.Path, err)
			continue
		}
		if !exists {
			skippedMissing++
			continue
		}
		if !drifted {
			continue
		}
		driftedCount++

		if !common.WhatIf(log.ToContext(cmd.Context()), cmd, "Updating skill at %s", inst.Path) {
			continue
		}

		if err := os.WriteFile(inst.Path, data, 0644); err != nil {
			return errors.Join(errors.Errorf("failed to write %s", inst.Path), err)
		}
		inst.Version = cmd.Root().Version
		fmt.Fprintf(out, "Updated %s\n", inst.Path)
		updated++
	}

	if skippedMissing > 0 {
		fmt.Fprintf(out, "Skipped %d missing tracked path(s); use \"bb skill forget\" to stop tracking them, or \"bb skill install\" to reinstall.\n", skippedMissing)
	}

	if updated == 0 {
		// Nothing was written. Distinguish "everything already current" from a
		// dry-run that previewed drifted installs but intentionally skipped them.
		if driftedCount == 0 {
			fmt.Fprintln(out, "All tracked skills are up to date.")
		}
		return nil
	}

	if err := state.Save(); err != nil {
		return err
	}
	fmt.Fprintf(out, "Updated %d skill installation(s).\n", updated)
	return nil
}

func forgetProcess(cmd *cobra.Command, args []string) error {
	state, err := LoadState()
	if err != nil {
		return err
	}

	path := args[0]
	removed := state.Remove(path)
	if !removed {
		fmt.Fprintf(cmd.OutOrStdout(), "%s was not tracked.\n", path)
		return nil
	}

	if err := state.Save(); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Stopped tracking %s\n", path)
	return nil
}
