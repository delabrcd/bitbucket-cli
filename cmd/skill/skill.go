package skill

import (
	"embed"
	"fmt"

	"github.com/spf13/cobra"
)

// skillName is the directory the skill is installed under, and matches the
// `name:` field of the embedded SKILL.md.
const skillName = "bitbucket-cli"

// skillAsset is the embed path of the bundled skill document.
const skillAsset = "assets/skills/bitbucket-cli/SKILL.md"

//go:embed all:assets
var assets embed.FS

// Command represents this folder's command
var Command = &cobra.Command{
	Use:   "skill",
	Short: "Install the bb agent skill",
	Long: `Manage the bb agent skill.

bb bundles a "skill" document that teaches an AI coding agent (e.g. Claude Code)
how to drive the bb CLI. "bb skill install" drops it into a Claude skills
directory.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Skill requires a subcommand:")
		for _, command := range cmd.Commands() {
			fmt.Println(command.Name())
		}
	},
}
