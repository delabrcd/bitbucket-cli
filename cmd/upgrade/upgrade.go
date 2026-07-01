package upgrade

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/delabrcd/bitbucket-cli/cmd/common"
	"github.com/gildas/go-errors"
	"github.com/gildas/go-logger"
	"github.com/spf13/cobra"
)

// Command is the top-level `bb upgrade` cobra command.
var Command = &cobra.Command{
	Use:     "upgrade",
	Aliases: []string{"update", "self-update"},
	Short:   "Upgrade bb to the latest release",
	Long: `Upgrade bb to the latest release from GitHub (delabrcd/bitbucket-cli).

On Linux, if the running binary lives under a system prefix (/usr/, /bin/, /opt/)
the command prints guidance to use your package manager (apt/dnf/pacman) instead.
Pass --force to bypass that check and replace the binary directly.

On Windows, the latest NSIS installer is downloaded and launched; follow the
wizard to complete the upgrade.

On macOS (and Linux with --force or a user-local install), the binary is replaced
in place: the release tar.gz is downloaded, its SHA-256 checksum is verified
against checksums.txt, the bb binary is extracted, and the running executable is
atomically replaced.

Use --check to only report whether a newer version is available without installing.`,
	Args: cobra.NoArgs,
	RunE: upgradeProcess,
}

var upgradeOptions struct {
	Check bool
	Force bool
}

func init() {
	Command.Flags().BoolVar(&upgradeOptions.Check, "check", false, "Report whether a newer version is available without installing")
	Command.Flags().BoolVar(&upgradeOptions.Force, "force", false, "On Linux, replace the binary even if it appears to be package-managed")
}

// githubRelease captures the fields we need from the GitHub releases API.
type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func upgradeProcess(cmd *cobra.Command, args []string) error {
	log := logger.Must(logger.FromContext(cmd.Context())).Child("upgrade", "upgrade")

	current := cmd.Root().Version
	log.Debugf("Current version: %s", current)

	// 1. Fetch latest release metadata from GitHub.
	release, err := fetchLatestRelease(log)
	if err != nil {
		return errors.Join(errors.Errorf("failed to fetch latest release"), err)
	}

	latest := strings.TrimPrefix(release.TagName, "v")

	fmt.Fprintf(os.Stderr, "Current: %s\nLatest:  %s\n", current, latest)

	if !isNewer(latest, current) {
		fmt.Fprintln(os.Stderr, "bb is up to date.")
		return nil
	}

	// 2. --check: just report availability.
	if upgradeOptions.Check {
		fmt.Fprintf(os.Stderr, "A newer version is available: %s\n", latest)
		return nil
	}

	// 3. Gate the actual install behind --dry-run / WhatIf.
	if !common.WhatIf(log.ToContext(cmd.Context()), cmd, "Upgrading bb from %s to %s", current, latest) {
		return nil
	}

	// 4. Route by OS.
	switch runtime.GOOS {
	case "windows":
		return installWindows(log, release, latest)
	case "linux":
		return installLinux(log, release, latest, upgradeOptions.Force)
	default: // darwin and anything else
		return installBinaryReplace(log, release, latest)
	}
}

// fetchLatestRelease calls the GitHub releases API and returns parsed metadata.
func fetchLatestRelease(log *logger.Logger) (*githubRelease, error) {
	const apiURL = "https://api.github.com/repos/delabrcd/bitbucket-cli/releases/latest"

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, errors.Join(errors.Errorf("failed to build release request"), err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "bb-cli")

	log.Debugf("GET %s", apiURL)
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Join(errors.Errorf("HTTP request failed"), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, errors.Join(errors.Errorf("failed to decode release JSON"), err)
	}
	return &release, nil
}

// isNewer returns true when latestStr represents a semver that is strictly
// greater than currentStr.  Both leading "v" prefixes have already been
// stripped by the caller.  If currentStr cannot be parsed as X.Y.Z (e.g. "dev"
// or a go pseudo-version) the function returns true so dev builds can move to a
// real release.
func isNewer(latestStr, currentStr string) bool {
	lv, ok := parseSemver(latestStr)
	if !ok {
		return false // can't parse latest — nothing sensible to do
	}
	cv, ok := parseSemver(currentStr)
	if !ok {
		return true // current is not a real release version; treat update as available
	}
	for i := range lv {
		if lv[i] > cv[i] {
			return true
		}
		if lv[i] < cv[i] {
			return false
		}
	}
	return false // equal
}

// parseSemver extracts the leading X.Y.Z integers from a version string,
// ignoring any -pre or +build suffix.  Returns ([major, minor, patch], true)
// or (nil, false) if the string doesn't start with three dot-separated integers.
func parseSemver(v string) ([3]int, bool) {
	// Strip a leading "v" (e.g. a git-describe / build-info version).
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	// Strip any pre-release or build-metadata suffix.
	for _, sep := range []string{"-", "+"} {
		if idx := strings.Index(v, sep); idx >= 0 {
			v = v[:idx]
		}
	}
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 3 {
		return [3]int{}, false
	}
	var nums [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return [3]int{}, false
		}
		nums[i] = n
	}
	return nums, true
}

// installWindows downloads the NSIS installer and launches it detached.
func installWindows(log *logger.Logger, release *githubRelease, latest string) error {
	assetName := ""
	downloadURL := ""
	for _, a := range release.Assets {
		if strings.HasSuffix(a.Name, "windows_amd64-setup.exe") {
			assetName = a.Name
			downloadURL = a.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		fmt.Fprintf(os.Stderr, "No Windows installer asset found for %s.\nDownload manually: https://github.com/delabrcd/bitbucket-cli/releases/latest\n", latest)
		return nil
	}

	log.Debugf("Downloading Windows installer %s", assetName)
	tmpPath := filepath.Join(os.TempDir(), assetName)
	if err := downloadToFile(downloadURL, tmpPath); err != nil {
		return errors.Join(errors.Errorf("failed to download installer"), err)
	}

	log.Infof("Launching installer %s", tmpPath)
	if err := exec.Command(tmpPath).Start(); err != nil {
		return errors.Join(errors.Errorf("failed to launch installer %s", tmpPath), err)
	}
	fmt.Fprintln(os.Stderr, "Launched the installer; follow the wizard to complete the upgrade.")
	return nil
}

// installLinux handles the Linux upgrade path: package-manager guidance unless
// the binary is user-local or --force is set.
func installLinux(log *logger.Logger, release *githubRelease, latest string, force bool) error {
	exePath, err := resolvedExecutable()
	if err != nil {
		return errors.Join(errors.Errorf("cannot determine executable path"), err)
	}

	if !force && isPackageManaged(exePath) {
		fmt.Fprintln(os.Stderr, "bb appears to be installed via a system package manager.")
		fmt.Fprintln(os.Stderr, "Update it with your package manager, for example:")
		fmt.Fprintln(os.Stderr, "  sudo apt update && sudo apt upgrade bitbucket-cli")
		fmt.Fprintln(os.Stderr, "  sudo dnf upgrade bitbucket-cli")
		fmt.Fprintln(os.Stderr, "  sudo pacman -Syu bitbucket-cli")
		fmt.Fprintln(os.Stderr, "Run with --force to replace the binary directly.")
		return nil
	}

	return installBinaryReplace(log, release, latest)
}

// installBinaryReplace performs the download-verify-extract-replace flow for
// Linux (non-package) and macOS.
func installBinaryReplace(log *logger.Logger, release *githubRelease, latest string) error {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	assetName := fmt.Sprintf("bitbucket-cli_%s_%s_%s.tar.gz", latest, goos, goarch)

	log.Debugf("Looking for asset %s", assetName)

	// Find the tar.gz asset and checksums.txt.
	tarURL := ""
	checksumURL := ""
	for _, a := range release.Assets {
		switch a.Name {
		case assetName:
			tarURL = a.BrowserDownloadURL
		case "checksums.txt":
			checksumURL = a.BrowserDownloadURL
		}
	}
	if tarURL == "" {
		return errors.Errorf("release asset %q not found in latest release", assetName)
	}
	if checksumURL == "" {
		return errors.Errorf("checksums.txt not found in latest release")
	}

	// Download tar.gz into memory.
	log.Debugf("Downloading %s", tarURL)
	tarData, err := downloadToMemory(tarURL)
	if err != nil {
		return errors.Join(errors.Errorf("failed to download %s", assetName), err)
	}

	// Download checksums.txt.
	log.Debugf("Downloading checksums.txt")
	checksumsData, err := downloadToMemory(checksumURL)
	if err != nil {
		return errors.Join(errors.Errorf("failed to download checksums.txt"), err)
	}

	// Verify SHA-256.
	if err := verifySHA256(tarData, assetName, checksumsData); err != nil {
		return errors.Join(errors.Errorf("checksum verification failed — aborting upgrade"), err)
	}
	log.Infof("Checksum verified for %s", assetName)

	// Extract the bb binary from the tar.gz.
	bbBytes, err := extractFromTarGz(tarData, "bb")
	if err != nil {
		return errors.Join(errors.Errorf("failed to extract bb from archive"), err)
	}

	// Determine the running executable path.
	exePath, err := resolvedExecutable()
	if err != nil {
		return errors.Join(errors.Errorf("cannot determine executable path"), err)
	}

	// Write to a temp file in the same directory for an atomic rename.
	exeDir := filepath.Dir(exePath)
	tmp, err := os.CreateTemp(exeDir, ".bb-upgrade-*")
	if err != nil {
		return errors.Join(errors.Errorf("failed to create temp file in %s", exeDir), err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }() // no-op after a successful rename

	if _, err := tmp.Write(bbBytes); err != nil {
		_ = tmp.Close()
		return errors.Join(errors.Errorf("failed to write new binary"), err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		return errors.Join(errors.Errorf("failed to chmod new binary"), err)
	}
	if err := tmp.Close(); err != nil {
		return errors.Join(errors.Errorf("failed to close temp file"), err)
	}

	// Atomic replace.
	if err := os.Rename(tmpPath, exePath); err != nil {
		if os.IsPermission(err) {
			return errors.Join(errors.Errorf("permission denied replacing %s — try re-running with sudo or use your package manager", exePath), err)
		}
		return errors.Join(errors.Errorf("failed to replace %s", exePath), err)
	}

	fmt.Fprintf(os.Stderr, "Upgraded bb to %s.\n", latest)
	return nil
}

// resolvedExecutable returns the real path of the current executable (symlinks
// resolved).
func resolvedExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// isPackageManaged returns true when path is under a well-known system prefix.
func isPackageManaged(path string) bool {
	for _, prefix := range []string{"/usr/", "/bin/", "/opt/"} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// downloadToMemory fetches a URL and returns the body as a byte slice.
func downloadToMemory(url string) ([]byte, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "bb-cli")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("HTTP %d fetching %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

// downloadToFile fetches a URL and writes the body to a local file path.
func downloadToFile(url, dest string) error {
	data, err := downloadToMemory(url)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0o755)
}

// verifySHA256 checks that the SHA-256 of data matches the line for assetName
// in checksumsData (format: "<hex>  <filename>" per line, as produced by
// sha256sum / GoReleaser).
func verifySHA256(data []byte, assetName string, checksumsData []byte) error {
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])

	for _, line := range strings.Split(string(checksumsData), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Each line: "<hex>  <filename>"
		fields := strings.SplitN(line, "  ", 2)
		if len(fields) != 2 {
			continue
		}
		if strings.TrimSpace(fields[1]) == assetName {
			want := strings.TrimSpace(fields[0])
			if got != want {
				return errors.Errorf("SHA-256 mismatch for %s: got %s, want %s", assetName, got, want)
			}
			return nil
		}
	}
	return errors.Errorf("no checksum entry found for %s in checksums.txt", assetName)
}

// extractFromTarGz finds entryName (at any path depth) in a gzip-compressed
// tar archive and returns its contents.
func extractFromTarGz(data []byte, entryName string) ([]byte, error) {
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, errors.Join(errors.Errorf("failed to open gzip stream"), err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.Join(errors.Errorf("error reading tar archive"), err)
		}
		if filepath.Base(hdr.Name) == entryName {
			return io.ReadAll(tr)
		}
	}
	return nil, errors.Errorf("entry %q not found in archive", entryName)
}
