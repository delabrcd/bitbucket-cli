package runner

import "github.com/spf13/cobra"

type Runners []Runner

// GetHeaders gets the header for a table
//
// implements common.Tableables
func (runners Runners) GetHeaders(cmd *cobra.Command) []string {
	return Runner{}.GetHeaders(cmd)
}

// GetRowAt gets the row for a table
//
// implements common.Tableables
func (runners Runners) GetRowAt(index int, headers []string) []string {
	if index < 0 || index >= len(runners) {
		return []string{}
	}
	return runners[index].GetRow(headers)
}

// Size gets the number of elements
//
// implements common.Tableables
func (runners Runners) Size() int {
	return len(runners)
}
