package cmd

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// Top-level command group identifiers.
const (
	groupCore     = "core"
	groupPipeline = "pipeline"
	groupConfig   = "config"
)

// commandGroups defines the ordered top-level command groups shown in the root help.
var commandGroups = []*cobra.Group{
	{ID: groupCore, Title: "CORE COMMANDS"},
	{ID: groupPipeline, Title: "PIPELINE COMMANDS"},
	{ID: groupConfig, Title: "CONFIGURATION COMMANDS"},
}

// commandGroupByName maps a top-level command name to the group it belongs to.
// Commands not listed here fall under "ADDITIONAL COMMANDS".
var commandGroupByName = map[string]string{
	"pullrequest": groupCore,
	"issue":       groupCore,
	"repo":        groupCore,
	"commit":      groupCore,
	"branch":      groupCore,
	"tag":         groupCore,

	"pipeline": groupPipeline,
	"runner":   groupPipeline,
	"artifact": groupPipeline,

	"profile":   groupConfig,
	"workspace": groupConfig,
	"project":   groupConfig,
	"component": groupConfig,
	"user":      groupConfig,
	"gpg-key":   groupConfig,
	"ssh-key":   groupConfig,
}

// registerCommandGroups declares the command groups on the root command and
// assigns each top-level command to its group.
func registerCommandGroups(root *cobra.Command) {
	root.AddGroup(commandGroups...)
	for _, c := range root.Commands() {
		if id, ok := commandGroupByName[c.Name()]; ok {
			c.GroupID = id
		}
	}
}

// setHelpAndUsage installs the custom help and usage renderers on the root
// command. Cobra propagates these to every subcommand.
func setHelpAndUsage(root *cobra.Command) {
	root.SetHelpFunc(helpFunc)
	root.SetUsageFunc(usageFunc)
}

type helpEntry struct {
	Title string
	Body  string
}

type helpCommandGroup struct {
	Title    string
	Commands []*cobra.Command
}

// groupedCommands returns the visible subcommands of cmd arranged into display
// groups. The root command uses the configured groups; any other command lists
// its subcommands under a single "AVAILABLE COMMANDS" heading.
func groupedCommands(cmd *cobra.Command) []helpCommandGroup {
	include := func(c *cobra.Command) bool {
		return c.IsAvailableCommand() && c.Name() != "help"
	}

	if cmd.HasParent() {
		var cmds []*cobra.Command
		for _, c := range cmd.Commands() {
			if include(c) {
				cmds = append(cmds, c)
			}
		}
		if len(cmds) == 0 {
			return nil
		}
		return []helpCommandGroup{{Title: "AVAILABLE COMMANDS", Commands: cmds}}
	}

	var groups []helpCommandGroup
	for _, g := range commandGroups {
		var cmds []*cobra.Command
		for _, c := range cmd.Commands() {
			if include(c) && c.GroupID == g.ID {
				cmds = append(cmds, c)
			}
		}
		if len(cmds) > 0 {
			groups = append(groups, helpCommandGroup{Title: g.Title, Commands: cmds})
		}
	}

	var additional []*cobra.Command
	for _, c := range cmd.Commands() {
		if include(c) && c.GroupID == "" {
			additional = append(additional, c)
		}
	}
	if len(additional) > 0 {
		groups = append(groups, helpCommandGroup{Title: "ADDITIONAL COMMANDS", Commands: additional})
	}
	return groups
}

// helpFunc renders the help output for any command.
func helpFunc(cmd *cobra.Command, _ []string) {
	bold := color.New(color.Bold).SprintFunc()

	var entries []helpEntry

	longText := cmd.Long
	if longText == "" {
		longText = cmd.Short
	}
	if longText != "" {
		entries = append(entries, helpEntry{Body: longText})
	}

	entries = append(entries, helpEntry{Title: "USAGE", Body: usageLine(cmd)})

	if len(cmd.Aliases) > 0 {
		entries = append(entries, helpEntry{Title: "ALIASES", Body: cmd.NameAndAliases()})
	}

	namePadding := 12
	for _, group := range groupedCommands(cmd) {
		for _, c := range group.Commands {
			if len(c.Name()) > namePadding-2 {
				namePadding = len(c.Name()) + 2
			}
		}
	}
	for _, group := range groupedCommands(cmd) {
		var lines []string
		for _, c := range group.Commands {
			lines = append(lines, rpad(c.Name()+":", namePadding)+c.Short)
		}
		entries = append(entries, helpEntry{Title: group.Title, Body: strings.Join(lines, "\n")})
	}

	if flagUsages := cmd.LocalFlags().FlagUsages(); flagUsages != "" {
		entries = append(entries, helpEntry{Title: "FLAGS", Body: dedent(flagUsages)})
	}
	if inherited := cmd.InheritedFlags().FlagUsages(); inherited != "" {
		entries = append(entries, helpEntry{Title: "INHERITED FLAGS", Body: dedent(inherited)})
	}

	if cmd.Example != "" {
		entries = append(entries, helpEntry{Title: "EXAMPLES", Body: cmd.Example})
	}

	entries = append(entries, helpEntry{Title: "LEARN MORE", Body: learnMore(cmd)})

	out := cmd.OutOrStdout()
	for i, e := range entries {
		if e.Title != "" {
			fmt.Fprintln(out, bold(e.Title))
			fmt.Fprintln(out, indent(strings.Trim(e.Body, "\r\n"), "  "))
		} else {
			fmt.Fprintln(out, strings.Trim(e.Body, "\r\n"))
		}
		if i < len(entries)-1 {
			fmt.Fprintln(out)
		}
	}
}

// usageFunc renders the short usage shown when a command is used incorrectly.
func usageFunc(cmd *cobra.Command) error {
	out := cmd.OutOrStderr()
	fmt.Fprintf(out, "Usage:  %s", usageLine(cmd))

	if groups := groupedCommands(cmd); len(groups) > 0 {
		fmt.Fprint(out, "\n\nAvailable commands:\n")
		for _, group := range groups {
			for _, c := range group.Commands {
				fmt.Fprintf(out, "  %s\n", c.Name())
			}
		}
		return nil
	}

	if flagUsages := cmd.LocalFlags().FlagUsages(); flagUsages != "" {
		fmt.Fprint(out, "\n\nFlags:\n")
		fmt.Fprint(out, indent(strings.TrimRight(dedent(flagUsages), "\n"), "  "))
		fmt.Fprintln(out)
	}
	return nil
}

// usageLine returns the usage string for a command. Commands that only group
// subcommands are shown as "<path> <command> [flags]"; runnable commands use
// their declared usage line.
func usageLine(cmd *cobra.Command) string {
	if cmd.HasAvailableSubCommands() && !cmd.Runnable() {
		line := cmd.CommandPath() + " <command>"
		if cmd.HasAvailableFlags() {
			line += " [flags]"
		}
		return line
	}
	return cmd.UseLine()
}

func learnMore(cmd *cobra.Command) string {
	return fmt.Sprintf(
		"Use `%s <command> <subcommand> --help` for more information about a command.\nRead the docs at https://github.com/delabrcd/bitbucket-cli/wiki",
		cmd.Root().Name(),
	)
}

func rpad(s string, padding int) string {
	return fmt.Sprintf("%-*s", padding, s)
}

// indent prefixes every non-empty line of s with prefix.
func indent(s, prefix string) string {
	if s == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}

// dedent removes the largest common leading-space indent from every line.
func dedent(s string) string {
	lines := strings.Split(s, "\n")
	minIndent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		n := len(line) - len(strings.TrimLeft(line, " "))
		if minIndent == -1 || n < minIndent {
			minIndent = n
		}
	}
	if minIndent <= 0 {
		return s
	}
	for i, line := range lines {
		if len(line) >= minIndent {
			lines[i] = line[minIndent:]
		}
	}
	return strings.Join(lines, "\n")
}
