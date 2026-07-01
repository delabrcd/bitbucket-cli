package skill

import (
	"os"
	"path/filepath"
	"testing"
)

// sandboxConfig points statePath() at a throwaway config dir for the duration
// of a test by overriding both HOME and XDG_CONFIG_HOME (covering the Linux and
// macOS resolutions of os.UserConfigDir).
func sandboxConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
}

func TestNormalizePath(t *testing.T) {
	rel := filepath.Join("a", "..", "b", "skill.md")
	got := normalizePath(rel)
	if !filepath.IsAbs(got) {
		t.Fatalf("normalizePath(%q) = %q, want absolute path", rel, got)
	}
	if filepath.Base(got) != "skill.md" || filepath.Base(filepath.Dir(got)) != "b" {
		t.Errorf("normalizePath(%q) = %q, want it cleaned to .../b/skill.md", rel, got)
	}
}

func TestRecordUpsert(t *testing.T) {
	var s State
	p := "/tmp/skills/bitbucket-cli/SKILL.md"

	s.Record(p, "1.0.0")
	if len(s.Installations) != 1 {
		t.Fatalf("after first Record: got %d installations, want 1", len(s.Installations))
	}
	firstStamp := s.Installations[0].InstalledAt

	// Re-recording the same (normalized) path updates in place, does not append.
	s.Record(p, "2.0.0")
	if len(s.Installations) != 1 {
		t.Fatalf("after re-Record of same path: got %d installations, want 1 (upsert)", len(s.Installations))
	}
	if s.Installations[0].Version != "2.0.0" {
		t.Errorf("upsert did not refresh version: got %q, want %q", s.Installations[0].Version, "2.0.0")
	}
	if !s.Installations[0].InstalledAt.After(firstStamp) && !s.Installations[0].InstalledAt.Equal(firstStamp) {
		t.Errorf("upsert should refresh InstalledAt to a not-earlier time")
	}

	// A different path appends a new entry.
	s.Record("/tmp/other/SKILL.md", "2.0.0")
	if len(s.Installations) != 2 {
		t.Fatalf("after Record of new path: got %d installations, want 2", len(s.Installations))
	}
}

func TestRemove(t *testing.T) {
	var s State
	p := "/tmp/skills/bitbucket-cli/SKILL.md"
	s.Record(p, "1.0.0")

	// Remove accepts an equivalent (un-normalized) form of the same path.
	if !s.Remove("/tmp/skills/../skills/bitbucket-cli/SKILL.md") {
		t.Fatalf("Remove of equivalent path returned false, want true")
	}
	if len(s.Installations) != 0 {
		t.Fatalf("after Remove: got %d installations, want 0", len(s.Installations))
	}
	if s.Remove(p) {
		t.Errorf("Remove of already-absent path returned true, want false")
	}
}

func TestIsDrifted(t *testing.T) {
	embedded, err := assets.ReadFile(skillAsset)
	if err != nil {
		t.Fatalf("reading embedded skill: %s", err)
	}
	dir := t.TempDir()

	// Missing file: not drifted, does not exist, no error.
	missing := filepath.Join(dir, "missing.md")
	drifted, exists, err := IsDrifted(missing)
	if err != nil {
		t.Fatalf("IsDrifted(missing) error: %s", err)
	}
	if exists || drifted {
		t.Errorf("IsDrifted(missing) = (drifted=%v, exists=%v), want (false, false)", drifted, exists)
	}

	// Exact copy of the embedded skill: exists, not drifted.
	same := filepath.Join(dir, "same.md")
	if err := os.WriteFile(same, embedded, 0644); err != nil {
		t.Fatalf("writing same.md: %s", err)
	}
	drifted, exists, err = IsDrifted(same)
	if err != nil {
		t.Fatalf("IsDrifted(same) error: %s", err)
	}
	if !exists || drifted {
		t.Errorf("IsDrifted(same) = (drifted=%v, exists=%v), want (false, true)", drifted, exists)
	}

	// Tampered copy: exists and drifted.
	tampered := filepath.Join(dir, "tampered.md")
	if err := os.WriteFile(tampered, append(append([]byte{}, embedded...), []byte("\n# extra\n")...), 0644); err != nil {
		t.Fatalf("writing tampered.md: %s", err)
	}
	drifted, exists, err = IsDrifted(tampered)
	if err != nil {
		t.Fatalf("IsDrifted(tampered) error: %s", err)
	}
	if !exists || !drifted {
		t.Errorf("IsDrifted(tampered) = (drifted=%v, exists=%v), want (true, true)", drifted, exists)
	}
}

func TestLoadStateDefaultsWhenMissing(t *testing.T) {
	sandboxConfig(t)

	s, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState on missing file returned error: %s", err)
	}
	if s.UpdateMode != ModeNotify {
		t.Errorf("default UpdateMode = %q, want %q", s.UpdateMode, ModeNotify)
	}
	if len(s.Installations) != 0 {
		t.Errorf("default state has %d installations, want 0", len(s.Installations))
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	sandboxConfig(t)

	original := &State{UpdateMode: ModeAuto}
	original.Record("/tmp/skills/bitbucket-cli/SKILL.md", "3.1.4")
	if err := original.Save(); err != nil {
		t.Fatalf("Save: %s", err)
	}

	loaded, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState after Save: %s", err)
	}
	if loaded.UpdateMode != ModeAuto {
		t.Errorf("round-trip UpdateMode = %q, want %q", loaded.UpdateMode, ModeAuto)
	}
	if len(loaded.Installations) != 1 {
		t.Fatalf("round-trip installations = %d, want 1", len(loaded.Installations))
	}
	if loaded.Installations[0].Version != "3.1.4" {
		t.Errorf("round-trip version = %q, want %q", loaded.Installations[0].Version, "3.1.4")
	}
	if loaded.Installations[0].Path != normalizePath("/tmp/skills/bitbucket-cli/SKILL.md") {
		t.Errorf("round-trip path = %q, want normalized form", loaded.Installations[0].Path)
	}
}

func TestLoadStateEmptyModeDefaults(t *testing.T) {
	sandboxConfig(t)

	// A file that omits update_mode should load as the notify default.
	path := statePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %s", err)
	}
	if err := os.WriteFile(path, []byte("installations: []\n"), 0644); err != nil {
		t.Fatalf("write: %s", err)
	}

	s, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState: %s", err)
	}
	if s.UpdateMode != ModeNotify {
		t.Errorf("UpdateMode with empty mode = %q, want %q", s.UpdateMode, ModeNotify)
	}
}
