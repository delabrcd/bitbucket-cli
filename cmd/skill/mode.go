package skill

import (
	"fmt"

	"github.com/gildas/go-errors"
	"github.com/spf13/cobra"
)

var modeCmd = &cobra.Command{
	Use:   "mode [notify|auto|off]",
	Short: "get or set the skill update-check mode",
	Long: `mode controls what bb does at startup when a tracked skill installation is
out of date:

  notify (default)  print a throttled (once per 24h) stderr notice
  auto               silently rewrite out-of-date tracked skills on startup
  off                never check

With no argument, the current mode is printed. With one argument, the mode is
updated.`,
	Args:      cobra.MaximumNArgs(1),
	ValidArgs: []string{ModeNotify, ModeAuto, ModeOff},
	RunE:      modeProcess,
}

func init() {
	Command.AddCommand(modeCmd)
}

func modeProcess(cmd *cobra.Command, args []string) error {
	state, err := LoadState()
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()

	if len(args) == 0 {
		fmt.Fprintf(out, "Update mode: %s\n", state.UpdateMode)
		return nil
	}

	requested := args[0]
	switch requested {
	case ModeNotify, ModeAuto, ModeOff:
		// valid
	default:
		return errors.Errorf("invalid mode %q; valid values are: %s, %s, %s", requested, ModeNotify, ModeAuto, ModeOff)
	}

	state.UpdateMode = requested
	if err := state.Save(); err != nil {
		return err
	}
	fmt.Fprintf(out, "Update mode set to: %s\n", state.UpdateMode)
	return nil
}
