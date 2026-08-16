package manager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// Search returns matching available versions.
func (m *Manager) Search(ctx context.Context, query string, limit int) ([]string, error) {
	return m.Source.List(ctx, query, limit)
}

// Install downloads, verifies and extracts a version.
func (m *Manager) Install(ctx context.Context, versionArg string) (*version.Artifact, error) {
	return m.InstallWithProgress(ctx, versionArg, nil)
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

	if fi, err := os.Stat(m.installPath(art.Version)); err == nil && fi.IsDir() {
		return nil, fmt.Errorf("%s %s is already installed", m.Kind, art.Version)
	}

	if err := m.Cfg.Ensure(); err != nil {
		return nil, err
	}

	// 1. download to cache (try primary URL first, then mirrors)
	cacheFile := filepath.Join(m.Cfg.CacheDir(), art.Filename)
	if progress != nil {
		progress(0, art.Size, 0)
	}
	if err := m.downloadWithMirrors(ctx, art, cacheFile, progress); err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	if progress != nil {
		progress(art.Size, art.Size, 0)
	}

	// 2. verify checksum
	if ok, err := m.verify(cacheFile, art); err != nil {
		return nil, err
	} else if !ok {
		return nil, fmt.Errorf("checksum verification failed for %s", cacheFile)
	}

	// 3. extract
	if err := extractArchive(cacheFile, m.toolDir()); err != nil {
		return nil, fmt.Errorf("extract failed: %w", err)
	}

	// Archives unpack with a wrapper dir ("jdk-<ver>" / "apache-maven-<ver>");
	// rename to the plain version for consistent naming.
	switch m.Kind {
	case KindMaven:
		from := filepath.Join(m.toolDir(), "apache-maven-"+art.Version)
		if _, err := os.Stat(from); err == nil {
			if err := os.Rename(from, m.installPath(art.Version)); err != nil {
				return nil, fmt.Errorf("rename %s: %w", from, err)
			}
		}
	case KindJDK:
		for _, candidate := range []string{"jdk-" + art.Version, "jdk" + art.Version} {
			from := filepath.Join(m.toolDir(), candidate)
			if _, err := os.Stat(from); err == nil {
				if err := os.Rename(from, m.installPath(art.Version)); err != nil {
					return nil, fmt.Errorf("rename %s: %w", from, err)
				}
				break
			}
		}
	}
	return art, nil
}

func (m *Manager) verify(path string, art *version.Artifact) (bool, error) {
	if art.SHA256 != "" {
		return download.VerifySHA256(path, art.SHA256)
	}
	return download.VerifySHA512(path, art.SHA512)
}

// downloadWithMirrors downloads from the primary URL, falling back to mirror
// URLs if the primary fails or is cancelled. Partial files are kept so each
// attempt resumes from where it left off.
func (m *Manager) downloadWithMirrors(ctx context.Context, art *version.Artifact, cacheFile string, progress ProgressFunc) error {
	urls := append([]string{art.URL}, art.Mirrors...)
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

// Current resolves the current version via the symlink.
func (m *Manager) Current() (string, error) {
	target, err := os.Readlink(m.currentSymlink())
	if err != nil {
		return "", err
	}
	return filepath.Base(target), nil
}

// Use switches the current version by updating the symlink.
func (m *Manager) Use(versionArg string) (string, error) {
	installed, err := m.Installed()
	if err != nil {
		return "", err
	}
	// resolve partial version against installed list
	var exact string
	for _, v := range installed {
		if v == versionArg || strings.HasPrefix(v, versionArg) || strings.HasPrefix(v, versionArg+".") {
			exact = v
			break
		}
	}
	if exact == "" {
		return "", fmt.Errorf("%s %s is not installed; run '%s %s install %s' first",
			m.Kind, versionArg, os.Args[0], m.Kind, versionArg)
	}
	if err := os.Symlink(m.installPath(exact), m.currentSymlink()); err != nil {
		if os.IsExist(err) {
			os.Remove(m.currentSymlink())
			if err := os.Symlink(m.installPath(exact), m.currentSymlink()); err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}
	return exact, nil
}

// Uninstall removes an installed version. If it is the current version, the
// "current" symlink and the shell environment block are cleaned up too.
func (m *Manager) Uninstall(versionArg string) error {
	installed, err := m.Installed()
	if err != nil {
		return err
	}
	var exact string
	for _, v := range installed {
		if v == versionArg {
			exact = v
			break
		}
	}
	if exact == "" {
		return fmt.Errorf("%s %s is not installed", m.Kind, versionArg)
	}

	wasCurrent := false
	if cur, err := m.Current(); err == nil && cur == exact {
		wasCurrent = true
		os.Remove(m.currentSymlink())
	}

	if err := os.RemoveAll(m.installPath(exact)); err != nil {
		return err
	}

	if wasCurrent {
		m.cleanupEnv()
	}
	return nil
}

// cleanupEnv removes the jm env block from the shell rc file so that
// JAVA_HOME / M2_HOME no longer point at a deleted install.
func (m *Manager) cleanupEnv() {
	rcFile := env.RCFile()
	changed, err := env.RemoveBlock(rcFile)
	if err != nil {
		fmt.Printf("提示: 清理环境变量失败: %v\n", err)
		return
	}
	if changed {
		fmt.Printf("已清理环境变量配置: %s\n", rcFile)
		fmt.Println("提示: 重新加载 shell 后 JAVA_HOME/M2_HOME 将失效，请重新使用 'jm <tool> use <版本>' 配置。")
	}
}

// Clean removes cached archives.
func (m *Manager) Clean() error {
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
