package update

import (
	"context"
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

	"jm/pkg/config"
)

const (
	repoOwner = "dowdsa"
	repoName  = "jvmtool"
)

// Release describes the latest release metadata.
type Release struct {
	TagName    string  `json:"tag_name"`
	HTMLURL    string  `json:"html_url"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}

// Asset describes a file attached to a GitHub release.
type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

// Version strips the leading "v" from a tag (e.g. "v0.3.0" -> "0.3.0").
func (r *Release) Version() string {
	return strings.TrimPrefix(r.TagName, "v")
}

// InstallerURL returns the desktop installer URL for the current platform.
// Desktop releases currently publish the Windows amd64 NSIS installer.
func (r *Release) InstallerURL(goos, goarch string) (string, error) {
	name := CLIInstallerAssetName(goos, goarch)
	for _, asset := range r.Assets {
		if asset.Name == name && asset.DownloadURL != "" {
			return asset.DownloadURL, nil
		}
	}
	return "", fmt.Errorf("release %s has no desktop installer for %s/%s", r.Version(), goos, goarch)
}

// DownloadInstaller downloads a release installer into the OS temp directory.
func DownloadInstaller(ctx context.Context, rel *Release, goos, goarch string) (string, error) {
	if goos != "windows" {
		return "", fmt.Errorf("desktop self-update is not supported on %s", goos)
	}
	url, err := rel.InstallerURL(goos, goarch)
	if err != nil {
		return "", err
	}
	client := config.HTTPClientWithTimeout(30 * time.Minute)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "jm-updater")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("installer download returned %s", resp.Status)
	}
	path := filepath.Join(os.TempDir(), "jm-update-"+rel.Version()+".exe")
	file, err := os.Create(path)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(file, resp.Body); err != nil {
		file.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	if err := verifyReleaseAsset(ctx, rel, CLIInstallerAssetName(goos, goarch), path); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func CLIInstallerAssetName(goos, goarch string) string {
	name := "jm-desktop_" + goos + "_" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

// CLIAssetName returns the release asset name for the current CLI platform.
func CLIAssetName(goos, goarch string) string {
	name := "jm_" + goos + "_" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

// DownloadCLI downloads the CLI executable for the specified platform.
func DownloadCLI(ctx context.Context, rel *Release, goos, goarch string) (string, error) {
	name := CLIAssetName(goos, goarch)
	var url string
	for _, asset := range rel.Assets {
		if asset.Name == name {
			url = asset.DownloadURL
			break
		}
	}
	if url == "" {
		return "", fmt.Errorf("release %s has no CLI asset for %s/%s", rel.Version(), goos, goarch)
	}
	client := config.HTTPClientWithTimeout(30 * time.Minute)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "jm-updater")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("CLI download returned %s", resp.Status)
	}
	path := filepath.Join(os.TempDir(), "jm-cli-update-"+rel.Version()+filepath.Ext(name))
	file, err := os.Create(path)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(file, resp.Body); err != nil {
		file.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	if goos != "windows" {
		if err := os.Chmod(path, 0o755); err != nil {
			_ = os.Remove(path)
			return "", err
		}
	}
	if err := verifyReleaseAsset(ctx, rel, name, path); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func verifyReleaseAsset(ctx context.Context, rel *Release, assetName, path string) error {
	var checksumURL string
	for _, asset := range rel.Assets {
		if asset.Name == "SHA256SUMS.txt" || asset.Name == "SHA256SUMS" {
			checksumURL = asset.DownloadURL
			break
		}
	}
	if checksumURL == "" {
		return fmt.Errorf("release %s has no SHA256SUMS asset", rel.Version())
	}
	client := config.HTTPClientWithTimeout(30 * time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "jm-updater")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("checksum download returned %s", resp.Status)
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	want := ""
	contents, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(contents), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.TrimLeft(fields[1], "*./") == assetName {
			want = strings.ToLower(fields[0])
			break
		}
	}
	if want == "" {
		return fmt.Errorf("checksum for %s not found", assetName)
	}
	if want != hex.EncodeToString(hasher.Sum(nil)) {
		return fmt.Errorf("SHA256 verification failed for %s", assetName)
	}
	return nil
}

// ApplyCLI replaces the running CLI with a downloaded executable. It exits
// the current process after scheduling the replacement.
func ApplyCLI(downloaded, executable string) error {
	if runtime.GOOS == "windows" {
		return scheduleWindowsReplacement(downloaded, executable)
	}
	backup := executable + ".old"
	_ = os.Remove(backup)
	if err := os.Rename(executable, backup); err != nil {
		return fmt.Errorf("move current CLI: %w", err)
	}
	if err := os.Rename(downloaded, executable); err != nil {
		_ = os.Rename(backup, executable)
		return fmt.Errorf("replace current CLI: %w", err)
	}
	_ = os.Remove(backup)
	return nil
}

func scheduleWindowsReplacement(downloaded, executable string) error {
	script := filepath.Join(os.TempDir(), "jm-cli-update-"+strconv.FormatInt(time.Now().UnixNano(), 10)+".ps1")
	content := `$ErrorActionPreference = "SilentlyContinue"
$source = $args[0]
$target = $args[1]
for ($i = 0; $i -lt 60; $i++) {
  Start-Sleep -Milliseconds 500
  try {
    Move-Item -LiteralPath $source -Destination $target -Force -ErrorAction Stop
    Remove-Item -LiteralPath $PSCommandPath -Force
    exit 0
  } catch {}
}
Remove-Item -LiteralPath $source -Force
Remove-Item -LiteralPath $PSCommandPath -Force`
	if err := os.WriteFile(script, []byte(content), 0o600); err != nil {
		return err
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-ExecutionPolicy", "Bypass", "-File", script, downloaded, executable)
	if err := cmd.Start(); err != nil {
		_ = os.Remove(script)
		return fmt.Errorf("start CLI updater: %w", err)
	}
	return nil
}

// Latest fetches the newest published, stable release from GitHub.
func Latest(ctx context.Context) (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=100", repoOwner, repoName)
	client := config.HTTPClientWithTimeout(15 * time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "jm-updater")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned %s", resp.Status)
	}
	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}
	var latest *Release
	for i := range releases {
		rel := &releases[i]
		if rel.Draft || rel.Prerelease || rel.Version() == "" {
			continue
		}
		if latest == nil || Compare(latest.Version(), rel.Version()) < 0 {
			latest = rel
		}
	}
	if latest == nil {
		return nil, fmt.Errorf("no stable published release found")
	}
	return latest, nil
}

// Compare compares two dotted version strings (e.g. "0.3.0" vs "0.3.1").
// Returns -1 if a < b, 0 if equal, 1 if a > b.
func Compare(a, b string) int {
	av := parseParts(a)
	bv := parseParts(b)
	n := len(av)
	if len(bv) > n {
		n = len(bv)
	}
	for i := 0; i < n; i++ {
		var x, y int
		if i < len(av) {
			x = av[i]
		}
		if i < len(bv) {
			y = bv[i]
		}
		if x < y {
			return -1
		}
		if x > y {
			return 1
		}
	}
	return 0
}

func parseParts(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		// strip non-numeric suffix like "-rc1", "+build"
		num := p
		if i := strings.IndexAny(p, "-+"); i >= 0 {
			num = p[:i]
		}
		n, err := strconv.Atoi(num)
		if err != nil {
			n = 0
		}
		out = append(out, n)
	}
	return out
}

// IsNewer reports whether latest is a newer version than current.
func IsNewer(current, latest string) bool {
	return Compare(current, latest) < 0
}
