package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// Cobra resolves a name/alias collision by registration order, which for commands
// registered from per-file init() functions is alphabetical by filename.
func TestNoSiblingNameCollisions(t *testing.T) {
	var walk func(parent *cobra.Command, path string)

	walk = func(parent *cobra.Command, path string) {
		claims := map[string]string{}

		for _, child := range parent.Commands() {
			for _, token := range append([]string{child.Name()}, child.Aliases...) {
				if owner, taken := claims[token]; taken {
					t.Errorf("%s: %q is claimed by both %q and %q", path, token, owner, child.Name())
					continue
				}
				claims[token] = child.Name()
			}
		}

		for _, child := range parent.Commands() {
			walk(child, strings.TrimSpace(path+" "+child.Name()))
		}
	}

	walk(RootCmd, "bb")
}

// remove/rm must only appear on a command that deletes something.
func TestDestructiveAliasesAreNotSharedWithNonDestructiveCommands(t *testing.T) {
	destructive := map[string]bool{"delete": true}

	var walk func(cmd *cobra.Command, path string)

	walk = func(cmd *cobra.Command, path string) {
		for _, alias := range cmd.Aliases {
			if alias != "rm" && alias != "remove" {
				continue
			}
			if !destructive[cmd.Name()] {
				t.Errorf("%s: non-destructive command %q must not offer the %q alias", path, cmd.Name(), alias)
			}
		}
		for _, child := range cmd.Commands() {
			walk(child, strings.TrimSpace(path+" "+child.Name()))
		}
	}

	walk(RootCmd, "bb")
}
