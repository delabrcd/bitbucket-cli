package skill

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestShouldSkipCheck(t *testing.T) {
	saved := os.Args
	t.Cleanup(func() { os.Args = saved })

	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"plain command", []string{"bb", "pr", "list"}, false},
		{"skill subcommand", []string{"bb", "skill", "list"}, true},
		{"completion subcommand", []string{"bb", "completion", "bash"}, true},
		{"dynamic completion", []string{"bb", "__complete", "pr", ""}, true},
		{"dynamic completion no desc", []string{"bb", "__completeNoDesc", "pr"}, true},
		{"global flag before skill", []string{"bb", "-o", "json", "skill", "list"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			os.Args = tc.args
			if got := shouldSkipCheck(); got != tc.want {
				t.Errorf("shouldSkipCheck(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

// installDrifted writes a tampered copy of the embedded skill at path and
// records it in a freshly-saved State with the given mode.
func installDrifted(t *testing.T, mode string) string {
	t.Helper()
	embedded, err := assets.ReadFile(skillAsset)
	if err != nil {
		t.Fatalf("reading embedded skill: %s", err)
	}
	skillFile := filepath.Join(t.TempDir(), "SKILL.md")
	tampered := append(append([]byte{}, embedded...), []byte("\n# drift\n")...)
	if err := os.WriteFile(skillFile, tampered, 0644); err != nil {
		t.Fatalf("writing skill file: %s", err)
	}

	s := &State{UpdateMode: mode}
	s.Record(skillFile, "old")
	if err := s.Save(); err != nil {
		t.Fatalf("saving state: %s", err)
	}
	return skillFile
}

func fileMatchesEmbedded(t *testing.T, path string) bool {
	t.Helper()
	drifted, exists, err := IsDrifted(path)
	if err != nil {
		t.Fatalf("IsDrifted(%s): %s", path, err)
	}
	return exists && !drifted
}

func TestMaybeCheckAutoRewrites(t *testing.T) {
	sandboxConfig(t)
	skillFile := installDrifted(t, ModeAuto)

	MaybeCheck(context.Background(), "9.9.9")

	if !fileMatchesEmbedded(t, skillFile) {
		t.Errorf("auto mode did not refresh the drifted skill at %s", skillFile)
	}
	// The registry entry's version should be bumped to the running version.
	s, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState: %s", err)
	}
	if len(s.Installations) != 1 || s.Installations[0].Version != "9.9.9" {
		t.Errorf("auto mode did not record new version: %+v", s.Installations)
	}
}

func TestMaybeCheckOffDoesNothing(t *testing.T) {
	sandboxConfig(t)
	skillFile := installDrifted(t, ModeOff)

	MaybeCheck(context.Background(), "9.9.9")

	if fileMatchesEmbedded(t, skillFile) {
		t.Errorf("off mode should not have touched the drifted skill at %s", skillFile)
	}
}

func TestMaybeCheckOptOutEnv(t *testing.T) {
	sandboxConfig(t)
	t.Setenv("BB_NO_SKILL_CHECK", "1")
	skillFile := installDrifted(t, ModeAuto)

	MaybeCheck(context.Background(), "9.9.9")

	if fileMatchesEmbedded(t, skillFile) {
		t.Errorf("BB_NO_SKILL_CHECK should have suppressed the auto update at %s", skillFile)
	}
}
