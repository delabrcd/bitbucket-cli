package skill

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gildas/bitbucket-cli/cmd/common"
	"github.com/gildas/go-errors"
	"github.com/gildas/go-logger"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install [flags]",
	Short: "install the bb skill into a Claude skills directory",
	Long: `install the bundled bb skill into a Claude skills directory.

With no --dir, the target is auto-detected: a project-level .claude/skills
directory (searched upward from the current directory) is preferred, otherwise
the personal skills directory ($CLAUDE_CONFIG_DIR or ~/.claude)/skills is used.
An existing skill is never overwritten unless --force is given.`,
	Args: cobra.NoArgs,
	RunE: installProcess,
}

var installOptions struct {
	Dir   string
	Force bool
}

func init() {
	Command.AddCommand(installCmd)

	installCmd.Flags().StringVar(&installOptions.Dir, "dir", "", "Skills directory to install into (overrides auto-detection)")
	installCmd.Flags().BoolVar(&installOptions.Force, "force", false, "Overwrite an existing skill")
	_ = installCmd.MarkFlagDirname("dir")
}

func installProcess(cmd *cobra.Command, args []string) error {
	log := logger.Must(logger.FromContext(cmd.Context())).Child(cmd.Parent().Name(), "install")

	data, err := assets.ReadFile(skillAsset)
	if err != nil {
		return errors.Join(errors.New("failed to read the bundled skill"), err)
	}

	skillsDir := installOptions.Dir
	if len(skillsDir) == 0 {
		detected := detectSkillsDirs()
		for _, dir := range detected {
			fmt.Fprintf(os.Stderr, "Detected skills directory: %s\n", dir)
		}
		skillsDir = chooseSkillsDir(detected)
	}
	if len(skillsDir) == 0 {
		return errors.New("could not determine a skills directory; pass --dir")
	}

	dest := filepath.Join(skillsDir, skillName, "SKILL.md")
	if _, err := os.Stat(dest); err == nil && !installOptions.Force {
		return errors.Errorf("%s already exists; use --force to overwrite", dest)
	}

	if !common.WhatIf(log.ToContext(cmd.Context()), cmd, "Installing skill %s to %s", skillName, dest) {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return errors.Join(errors.Errorf("failed to create %s", filepath.Dir(dest)), err)
	}
	if err := os.WriteFile(dest, data, 0644); err != nil {
		return errors.Join(errors.Errorf("failed to write %s", dest), err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Installed skill %q to %s\n", skillName, dest)
	return nil
}

// detectSkillsDirs returns the skills directories that already exist, in
// preference order: project-level first, then personal.
func detectSkillsDirs() (dirs []string) {
	if project := findProjectSkillsDir(); len(project) > 0 {
		dirs = append(dirs, project)
	}
	if personal := personalSkillsDir(); len(personal) > 0 {
		if _, err := os.Stat(personal); err == nil {
			dirs = append(dirs, personal)
		}
	}
	return dirs
}

// chooseSkillsDir picks the install target: the first detected existing
// directory, or the personal directory as a fallback (created on write).
func chooseSkillsDir(detected []string) string {
	if len(detected) > 0 {
		return detected[0]
	}
	return personalSkillsDir()
}

// personalSkillsDir is ($CLAUDE_CONFIG_DIR or ~/.claude)/skills.
func personalSkillsDir() string {
	base := os.Getenv("CLAUDE_CONFIG_DIR")
	if len(base) == 0 {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".claude")
	}
	return filepath.Join(base, "skills")
}

// findProjectSkillsDir walks up from the current directory looking for a
// project-level .claude directory, returning its skills subdirectory. The walk
// stops at the home directory so that ~/.claude is treated as personal (handled
// by personalSkillsDir), not as a project match.
func findProjectSkillsDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	home, _ := os.UserHomeDir()
	for {
		if len(home) > 0 && dir == home {
			return ""
		}
		candidate := filepath.Join(dir, ".claude")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return filepath.Join(candidate, "skills")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
