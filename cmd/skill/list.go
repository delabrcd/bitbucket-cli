package skill

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls", "status"},
	Short:   "list tracked bb skill installations and their update status",
	Long: `list the bb skill installations that are tracked in the local registry,
along with their recorded version and whether the installed file still
matches the version of bb currently running.`,
	Args: cobra.NoArgs,
	RunE: listProcess,
}

func init() {
	Command.AddCommand(listCmd)
}

func listProcess(cmd *cobra.Command, args []string) error {
	state, err := LoadState()
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Update mode: %s\n", state.UpdateMode)

	if len(state.Installations) == 0 {
		fmt.Fprintln(out, "No tracked skill installations.")
		return nil
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PATH\tVERSION\tSTATUS")
	for _, inst := range state.Installations {
		status := "up-to-date"
		drifted, exists, err := IsDrifted(inst.Path)
		switch {
		case err != nil:
			status = fmt.Sprintf("ERROR: %s", err)
		case !exists:
			status = "MISSING"
		case drifted:
			status = "OUT OF DATE"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", inst.Path, inst.Version, status)
	}
	return w.Flush()
}
