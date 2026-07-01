package skill

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"time"

	"github.com/gildas/go-errors"
	"gopkg.in/yaml.v3"
)

// UpdateMode values.
const (
	ModeNotify = "notify" // default: print a throttled stderr notice when a tracked skill is out of date
	ModeAuto   = "auto"   // silently rewrite out-of-date tracked skills on startup
	ModeOff    = "off"    // never check
)

// Installation records one place the bb skill has been installed to.
type Installation struct {
	Path        string    `yaml:"path"`    // absolute path to the installed SKILL.md
	Version     string    `yaml:"version"` // bb version at install time (for display)
	InstalledAt time.Time `yaml:"installed_at"`
}

// State is the persisted skill-install registry.
type State struct {
	UpdateMode    string         `yaml:"update_mode"`
	LastNotified  time.Time      `yaml:"last_notified,omitempty"`
	Installations []Installation `yaml:"installations"`
}

// statePath returns the path to the persisted skill registry: under
// <os.UserConfigDir()>/bitbucket/skills.yaml, falling back to
// <home>/.bitbucket-skills.yaml if UserConfigDir errors, mirroring the
// fallback logic in cmd/common/config.go.
func statePath() string {
	if configDir, err := os.UserConfigDir(); err == nil && len(configDir) > 0 {
		return filepath.Join(configDir, "bitbucket", "skills.yaml")
	}
	home, err := os.UserHomeDir()
	if err != nil || len(home) == 0 {
		return ".bitbucket-skills.yaml"
	}
	return filepath.Join(home, ".bitbucket-skills.yaml")
}

// LoadState reads and parses the skill registry. A missing file is not an
// error: a fresh State with the default update mode is returned instead.
func LoadState() (*State, error) {
	path := statePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{UpdateMode: ModeNotify}, nil
		}
		return nil, errors.Join(errors.Errorf("failed to read %s", path), err)
	}

	var state State
	if err := yaml.Unmarshal(data, &state); err != nil {
		return nil, errors.Join(errors.Errorf("failed to parse %s", path), err)
	}
	if len(state.UpdateMode) == 0 {
		state.UpdateMode = ModeNotify
	}
	return &state, nil
}

// Save writes the registry back to disk, creating its parent directory if
// needed.
func (s *State) Save() error {
	path := statePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return errors.Join(errors.Errorf("failed to create %s", filepath.Dir(path)), err)
	}

	data, err := yaml.Marshal(s)
	if err != nil {
		return errors.Join(errors.New("failed to marshal skill registry"), err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return errors.Join(errors.Errorf("failed to write %s", path), err)
	}
	return nil
}

// normalizePath returns the cleaned absolute form of path, used as the
// registry's dedup key. If the path cannot be made absolute, the original
// (cleaned) input is returned instead.
func normalizePath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(path)
}

// Record upserts an Installation by its (normalized) path: an existing entry
// has its Version and InstalledAt refreshed in place; otherwise a new entry
// is appended.
func (s *State) Record(path, version string) {
	key := normalizePath(path)
	for i := range s.Installations {
		if s.Installations[i].Path == key {
			s.Installations[i].Version = version
			s.Installations[i].InstalledAt = time.Now()
			return
		}
	}
	s.Installations = append(s.Installations, Installation{
		Path:        key,
		Version:     version,
		InstalledAt: time.Now(),
	})
}

// Remove drops a tracked path from the registry, returning whether an entry
// was actually removed.
func (s *State) Remove(path string) bool {
	key := normalizePath(path)
	for i := range s.Installations {
		if s.Installations[i].Path == key {
			s.Installations = append(s.Installations[:i], s.Installations[i+1:]...)
			return true
		}
	}
	return false
}

// EmbeddedHash returns the SHA-256 hex digest of the embedded skill document.
func EmbeddedHash() (string, error) {
	data, err := assets.ReadFile(skillAsset)
	if err != nil {
		return "", errors.Join(errors.New("failed to read the bundled skill"), err)
	}
	return hashBytes(data), nil
}

// fileHash returns the SHA-256 hex digest of a file's contents.
func fileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", errors.Join(errors.Errorf("failed to read %s", path), err)
	}
	return hashBytes(data), nil
}

// hashBytes returns the SHA-256 hex digest of data.
func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// IsDrifted reports whether the installed skill at path differs from the
// embedded skill. exists is false (with no error) if path does not exist.
func IsDrifted(path string) (drifted bool, exists bool, err error) {
	if _, statErr := os.Stat(path); statErr != nil {
		if os.IsNotExist(statErr) {
			return false, false, nil
		}
		return false, false, errors.Join(errors.Errorf("failed to stat %s", path), statErr)
	}

	installedHash, err := fileHash(path)
	if err != nil {
		return false, true, err
	}
	embeddedHash, err := EmbeddedHash()
	if err != nil {
		return false, true, err
	}
	return installedHash != embeddedHash, true, nil
}
