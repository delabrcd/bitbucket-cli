package completion

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/delabrcd/bitbucket-cli/cmd/common"
	"github.com/gildas/go-errors"
	"github.com/gildas/go-flags"
	"github.com/gildas/go-logger"
	"github.com/spf13/cobra"
)

// InstallCmd is attached at runtime to cobra's auto-generated "completion"
// command (see cmd/root.go), so it appears as "bb completion install".
var InstallCmd = &cobra.Command{
	Use:   "install [flags]",
	Short: "install shell completions into the right directory",
	Long: `install shell completions into the directory the shell loads them from.

The shell is detected from $SHELL unless --shell is given, and the target
directory is auto-detected unless --dir is given:
  bash → ${BASH_COMPLETION_USER_DIR or XDG_DATA_HOME or ~/.local/share}/bash-completion/completions/bb
  zsh  → ${XDG_DATA_HOME or ~/.local/share}/zsh/site-functions/_bb
  fish → ${XDG_CONFIG_HOME or ~/.config}/fish/completions/bb.fish`,
	Args: cobra.NoArgs,
	RunE: installProcess,
}

var installOptions struct {
	Shell *flags.EnumFlag
	Dir   string
}

func init() {
	installOptions.Shell = flags.NewEnumFlag("bash", "zsh", "fish")
	InstallCmd.Flags().Var(installOptions.Shell, "shell", "Shell to install completions for (default: detected from $SHELL)")
	InstallCmd.Flags().StringVar(&installOptions.Dir, "dir", "", "Directory to install into (overrides auto-detection)")
	_ = InstallCmd.MarkFlagDirname("dir")
	_ = InstallCmd.RegisterFlagCompletionFunc(installOptions.Shell.CompletionFunc("shell"))
}

func installProcess(cmd *cobra.Command, args []string) error {
	log := logger.Must(logger.FromContext(cmd.Context())).Child(cmd.Parent().Name(), "install")

	shell := installOptions.Shell.Value
	if len(shell) == 0 {
		shell = detectShell()
	}

	dir := installOptions.Dir
	if len(dir) == 0 {
		dir = completionDir(shell)
	}
	if len(dir) == 0 {
		return errors.New("could not determine a completion directory; pass --dir")
	}
	dest := filepath.Join(dir, completionFilename(shell))

	if !common.WhatIf(log.ToContext(cmd.Context()), cmd, "Installing %s completions to %s", shell, dest) {
		return nil
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return errors.Join(errors.Errorf("failed to create %s", dir), err)
	}
	file, err := os.Create(dest)
	if err != nil {
		return errors.Join(errors.Errorf("failed to create %s", dest), err)
	}
	defer file.Close()

	root := cmd.Root()
	switch shell {
	case "bash":
		err = root.GenBashCompletionV2(file, true)
	case "zsh":
		err = root.GenZshCompletion(file)
	case "fish":
		err = root.GenFishCompletion(file, true)
	default:
		return errors.Errorf("unsupported shell %q", shell)
	}
	if err != nil {
		return errors.Join(errors.Errorf("failed to generate %s completions", shell), err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Installed %s completions to %s\n", shell, dest)
	if shell == "zsh" {
		fmt.Fprintf(cmd.OutOrStdout(), "Ensure %s is on your $fpath (e.g. add `fpath=(%s $fpath)` before `compinit` in ~/.zshrc).\n", dir, dir)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Start a new shell for completions to take effect.")
	return nil
}

// detectShell guesses the shell from $SHELL, defaulting to bash.
func detectShell() string {
	switch base := filepath.Base(os.Getenv("SHELL")); {
	case strings.Contains(base, "zsh"):
		return "zsh"
	case strings.Contains(base, "fish"):
		return "fish"
	default:
		return "bash"
	}
}

func completionFilename(shell string) string {
	switch shell {
	case "zsh":
		return "_bb"
	case "fish":
		return "bb.fish"
	default:
		return "bb"
	}
}

// completionDir returns the conventional user completion directory per shell.
func completionDir(shell string) string {
	switch shell {
	case "zsh":
		return filepath.Join(dataHome(), "zsh", "site-functions")
	case "fish":
		return filepath.Join(configHome(), "fish", "completions")
	default: // bash
		if d := os.Getenv("BASH_COMPLETION_USER_DIR"); len(d) > 0 {
			return filepath.Join(d, "completions")
		}
		return filepath.Join(dataHome(), "bash-completion", "completions")
	}
}

func dataHome() string {
	if d := os.Getenv("XDG_DATA_HOME"); len(d) > 0 {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share")
}

func configHome() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); len(d) > 0 {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}
