package manager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"jm/pkg/config"
	"jm/pkg/download"
	"jm/pkg/env"
	"jm/pkg/version"
)

// Kind identifies a managed tool family.
type Kind string

const (
	KindJDK   Kind = "jdk"
	KindMaven Kind = "maven"
)

// Manager orchestrates installation and version switching.
type Manager struct {
	Cfg    *config.Config
	Source version.Source
	Kind   Kind
	dl     *download.Downloader
}

func NewManager(cfg *config.Config, kind Kind) *Manager {
	var src version.Source
	switch kind {
	case KindJDK:
		src = version.NewJDKSource()
	case KindMaven:
		src = version.NewMavenSource()
	}
	return &Manager{
		Cfg:    cfg,
		Source: src,
		Kind:   kind,
		dl:     download.New(),
	}
}

func (m *Manager) toolDir() string {
	if m.Kind == KindJDK {
		return m.Cfg.JDKDir()
	}
	return m.Cfg.MavenDir()
}

func (m *Manager) installPath(ver string) string {
	return filepath.Join(m.toolDir(), ver)
}

func (m *Manager) currentSymlink() string {
	return filepath.Join(m.toolDir(), "current")
}

func (m *Manager) lock(ctx context.Context) (func(), error) {
	if err := m.Cfg.Ensure(); err != nil {
		return nil, err
	}
	path := filepath.Join(m.Cfg.Root, ".jm.lock")
	deadline := time.Now().Add(30 * time.Second)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_ = f.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if time.Now().After(deadline) {
			return nil, errors.New("timed out waiting for jm lock")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// Search returns matching available versions.
func (m *Manager) Search(ctx context.Context, query string, limit int) ([]string, error) {
	return m.Source.List(ctx, query, limit)
}

// Resolve resolves a (possibly partial) version to a concrete Artifact.
func (m *Manager) Resolve(ctx context.Context, versionArg string) (*version.Artifact, error) {
	return m.Source.Resolve(ctx, versionArg)
}

// CacheFile returns the cache path for an artifact.
func (m *Manager) CacheFile(art *version.Artifact) string {
	return filepath.Join(m.Cfg.CacheDir(), art.Filename)
}

// RemoveCache removes a partially-downloaded cache file.
func (m *Manager) RemoveCache(art *version.Artifact) error {
	return os.Remove(m.CacheFile(art))
}

// Install downloads, verifies and extracts a version. When called from a
// terminal (CLI), it renders a progress bar to stdout.
func (m *Manager) Install(ctx context.Context, versionArg string) (*version.Artifact, error) {
	bar := download.NewProgressBar(string(m.Kind) + " " + versionArg)
	art, err := m.InstallWithProgress(ctx, versionArg, ProgressFunc(bar.Callback()))
	bar.Done()
	return art, err
}

// ProgressFunc reports download progress (done, total, rate) in bytes.
type ProgressFunc func(done, total, rate int64)

// InstallWithProgress downloads, verifies and extracts a version, reporting
// download progress via progress if non-nil.
func (m *Manager) InstallWithProgress(ctx context.Context, versionArg string, progress ProgressFunc) (*version.Artifact, error) {
	art, err := m.Source.Resolve(ctx, versionArg)
	if err != nil {
		return nil, err
	}
	release, err := m.lock(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	if fi, err := os.Stat(m.installPath(art.Version)); err == nil && fi.IsDir() {
		return nil, fmt.Errorf("%s %s is already installed", m.Kind, art.Version)
	}

	if err := m.Cfg.Ensure(); err != nil {
		return nil, err
	}

	// 1. download to cache (try primary URL first, then mirrors)
	cacheFile := m.CacheFile(art)
	if progress != nil {
		progress(0, art.Size, 0)
	}
	if err := m.downloadWithMirrors(ctx, art, cacheFile, progress); err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	if progress != nil {
		progress(art.Size, art.Size, 0)
	}

	// 2. verify checksum; on failure drop the cached file so a retry
	// re-downloads instead of failing forever against the same corrupt file.
	if ok, err := m.verify(cacheFile, art); err != nil {
		return nil, err
	} else if !ok {
		os.Remove(cacheFile)
		return nil, fmt.Errorf("checksum verification failed for %s（已删除缓存，请重试）", cacheFile)
	}

	// 3. extract
	if err := extractArchive(cacheFile, m.toolDir()); err != nil {
		return nil, fmt.Errorf("extract failed: %w", err)
	}

	// Archives unpack with a wrapper dir ("jdk-<ver>" / "apache-maven-<ver>");
	// rename to the plain version for consistent naming.
	renameFailed := func(from string, err error) (*version.Artifact, error) {
		// remove the half-extracted wrapper dir instead of leaving junk
		os.RemoveAll(from)
		return nil, fmt.Errorf("rename %s: %w", from, err)
	}
	renamed := false
	switch m.Kind {
	case KindMaven:
		from := filepath.Join(m.toolDir(), "apache-maven-"+art.Version)
		if _, err := os.Stat(from); err == nil {
			if err := os.Rename(from, m.installPath(art.Version)); err != nil {
				return renameFailed(from, err)
			}
			renamed = true
		}
	case KindJDK:
		for _, candidate := range []string{"jdk-" + art.Version, "jdk" + art.Version} {
			from := filepath.Join(m.toolDir(), candidate)
			if _, err := os.Stat(from); err == nil {
				if err := os.Rename(from, m.installPath(art.Version)); err != nil {
					return renameFailed(from, err)
				}
				renamed = true
				break
			}
		}
	}
	if !renamed {
		return nil, fmt.Errorf("unexpected archive layout for %s %s", m.Kind, art.Version)
	}
	return art, nil
}

func (m *Manager) verify(path string, art *version.Artifact) (bool, error) {
	if art.SHA256 != "" {
		return download.VerifySHA256(path, art.SHA256)
	}
	if art.SHA512 == "" {
		return false, fmt.Errorf("no checksum provided for %s", art.Version)
	}
	return download.VerifySHA512(path, art.SHA512)
}

// downloadWithMirrors downloads preferring faster mirrors first, falling back
// to the primary URL. Partial files are kept so each attempt resumes from
// where it left off.
func (m *Manager) downloadWithMirrors(ctx context.Context, art *version.Artifact, cacheFile string, progress ProgressFunc) error {
	// mirrors first (typically faster), primary URL as fallback
	urls := append(append([]string{}, art.Mirrors...), art.URL)
	var lastErr error
	for _, u := range urls {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := m.dl.Download(ctx, u, cacheFile, download.ProgressCallback(progress)); err != nil {
			lastErr = err
			if ctx.Err() != nil {
				// cancelled: do not fall through to mirrors
				return err
			}
			continue
		}
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("all download sources failed")
	}
	return lastErr
}

// Installed returns installed versions (excluding "current").
func (m *Manager) Installed() ([]string, error) {
	entries, err := os.ReadDir(m.toolDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.Name() == "current" {
			continue
		}
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

// Current resolves the current version via the symlink. On Windows, if the
// symlink is missing (no admin/dev mode), it falls back to the env var.
func (m *Manager) Current() (string, error) {
	target, err := os.Readlink(m.currentSymlink())
	if err == nil {
		return filepath.Base(target), nil
	}
	if env.IsWindows() {
		key := "JAVA_HOME"
		if m.Kind == KindMaven {
			key = "M2_HOME"
		}
		// Read the persistent user value from the registry instead of
		// os.Getenv: in a long-running process (GUI) the process env is a
		// snapshot from startup and would be stale after SetUserEnvVar.
		if v, rerr := env.GetUserEnvVar(key); rerr == nil && v != "" {
			if strings.HasPrefix(filepath.Clean(v), filepath.Clean(m.toolDir())+string(os.PathSeparator)) {
				return filepath.Base(v), nil
			}
		}
	}
	return "", err
}

// Use switches the current version by updating the symlink.
func (m *Manager) Use(versionArg string) (string, error) {
	release, err := m.lock(context.Background())
	if err != nil {
		return "", err
	}
	defer release()

	installed, err := m.Installed()
	if err != nil {
		return "", err
	}
	// resolve partial version against installed list
	exact := matchInstalled(installed, versionArg)
	if exact == "" {
		return "", fmt.Errorf("%s %s is not installed; run '%s %s install %s' first",
			m.Kind, versionArg, os.Args[0], m.Kind, versionArg)
	}
	previous, _ := m.Current()
	tmpLink := m.currentSymlink() + ".tmp"
	_ = os.Remove(tmpLink)
	if err := os.Symlink(m.installPath(exact), tmpLink); err != nil {
		if !env.IsWindows() {
			return "", err
		}
	} else {
		if env.IsWindows() {
			_ = os.Remove(m.currentSymlink())
		}
		if err := os.Rename(tmpLink, m.currentSymlink()); err != nil {
			_ = os.Remove(tmpLink)
			return "", err
		}
	}
	if env.IsWindows() {
		if previous != "" && previous != exact {
			_ = env.RemovePathEntry(filepath.Join(m.installPath(previous), "bin"))
		}
		if err := env.AddPath(filepath.Join(m.installPath(exact), "bin")); err != nil {
			return "", err
		}
	}
	// Update the environment so new terminals pick up the change.
	m.applyEnv(exact)
	return exact, nil
}

// applyEnv updates persistent environment variables for the active version.
// On Windows it writes the user env vars (JAVA_HOME/M2_HOME) and updates PATH;
// on Unix it writes the shell rc block.
func (m *Manager) applyEnv(version string) {
	if env.IsWindows() {
		bin := filepath.Join(m.installPath(version), "bin")
		switch m.Kind {
		case KindJDK:
			env.SetUserEnvVar("JAVA_HOME", m.installPath(version))
		case KindMaven:
			env.SetUserEnvVar("M2_HOME", m.installPath(version))
			env.SetUserEnvVar("MAVEN_HOME", m.installPath(version))
		}
		env.AddPath(bin)
		return
	}
	if err := env.ApplyBlock(m.Cfg.Root); err != nil {
		fmt.Printf("提示: 写入环境变量配置失败: %v\n", err)
	}
}

// Uninstall removes an installed version. If it is the current version, the
// "current" symlink and the shell environment block are cleaned up too.
func (m *Manager) Uninstall(versionArg string) (string, error) {
	release, err := m.lock(context.Background())
	if err != nil {
		return "", err
	}
	defer release()

	installed, err := m.Installed()
	if err != nil {
		return "", err
	}
	// support partial version arguments, mirroring `use`
	exact := matchInstalled(installed, versionArg)
	if exact == "" {
		return "", fmt.Errorf("%s %s is not installed", m.Kind, versionArg)
	}

	wasCurrent := false
	if cur, err := m.Current(); err == nil && cur == exact {
		wasCurrent = true
		os.Remove(m.currentSymlink())
	}

	if err := os.RemoveAll(m.installPath(exact)); err != nil {
		return "", err
	}

	if wasCurrent {
		m.cleanupEnv(exact)
	}
	return exact, nil
}

// cleanupEnv removes the environment configuration pointing at a deleted
// install. On Unix it removes the jm block from the shell rc file; on Windows
// it clears the matching user registry vars and removes the bin dir from the
// user PATH.
func (m *Manager) cleanupEnv(version string) {
	if env.IsWindows() {
		home := m.installPath(version)
		switch m.Kind {
		case KindJDK:
			clearUserEnvIfMatches("JAVA_HOME", home)
		case KindMaven:
			clearUserEnvIfMatches("M2_HOME", home)
			clearUserEnvIfMatches("MAVEN_HOME", home)
		}
		if err := env.RemovePathEntry(filepath.Join(home, "bin")); err != nil {
			fmt.Printf("提示: 清理 PATH 失败: %v\n", err)
		}
		return
	}
	if err := env.ApplyBlock(m.Cfg.Root); err != nil {
		fmt.Printf("提示: 清理环境变量失败: %v\n", err)
	}
}

// clearUserEnvIfMatches deletes a user environment variable when its current
// value points at path (compared case-insensitively).
func clearUserEnvIfMatches(name, path string) {
	v, err := env.GetUserEnvVar(name)
	if err != nil || v == "" {
		return
	}
	if strings.EqualFold(filepath.Clean(v), filepath.Clean(path)) {
		_ = env.SetUserEnvVar(name, "")
	}
}

// matchInstalled resolves a (possibly partial) version argument against the
// installed list: exact match, plain prefix match, or prefix-plus-dot match.
func matchInstalled(installed []string, arg string) string {
	for _, v := range installed {
		if v == arg || strings.HasPrefix(v, arg) || strings.HasPrefix(v, arg+".") {
			return v
		}
	}
	return ""
}

// Clean removes cached archives.
func (m *Manager) Clean() error {
	release, err := m.lock(context.Background())
	if err != nil {
		return err
	}
	defer release()
	return os.RemoveAll(m.Cfg.CacheDir())
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
