package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	goruntime "runtime"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"jm/pkg/config"
	"jm/pkg/manager"
	"jm/pkg/update"
	"jm/pkg/version"
)

// App struct
type App struct {
	ctx context.Context
	cfg *config.Config

	mu           sync.Mutex
	cancel       context.CancelFunc
	busy         bool
	busyKey      string
	currentCache string
	stopMode     string    // "pause" | "cancel" — how the current download was stopped
	lastEmit     time.Time // throttle install:progress events
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{cfg: config.Default()}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if err := a.cfg.Ensure(); err != nil {
		fmt.Printf("ensure dirs: %v\n", err)
	}
	a.cfg.LoadSettings()
}

// Root returns the JVMTOOL_HOME root directory.
func (a *App) Root() string {
	return a.cfg.Root
}

// GetProxy returns the configured proxy string ("" if none).
func (a *App) GetProxy() string {
	return a.cfg.GetProxy()
}

// SetProxy persists the proxy setting. Empty string clears it.
func (a *App) SetProxy(proxy string) error {
	return a.cfg.SaveProxy(proxy)
}

// GetVersion returns the current app version.
func (a *App) GetVersion() string {
	return version.Version
}

// UpdateInfo describes an available update.
type UpdateInfo struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	Error   string `json:"error,omitempty"`
}

// CheckUpdate checks for a newer version. Returns empty Version if up-to-date.
func (a *App) CheckUpdate() UpdateInfo {
	rel, err := update.Latest(a.ctx)
	if err != nil {
		return UpdateInfo{Error: err.Error()}
	}
	latest := rel.Version()
	if !update.IsNewer(version.Version, latest) {
		return UpdateInfo{}
	}
	return UpdateInfo{Version: latest, URL: rel.HTMLURL}
}

// InstallUpdate downloads and launches the current platform's desktop
// installer. The installer replaces the old application after it exits.
func (a *App) InstallUpdate() error {
	if goruntime.GOOS != "windows" {
		return fmt.Errorf("桌面端自动更新暂不支持 %s", goruntime.GOOS)
	}
	rel, err := update.Latest(a.ctx)
	if err != nil {
		return err
	}
	path, err := update.DownloadInstaller(a.ctx, rel, goruntime.GOOS, goruntime.GOARCH)
	if err != nil {
		return err
	}
	if err := exec.Command(path).Start(); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("启动安装程序失败: %w", err)
	}
	// Give the installer a moment to initialize before closing the running
	// executable, which otherwise may still be locked on Windows.
	go func() {
		time.Sleep(800 * time.Millisecond)
		os.Exit(0)
	}()
	return nil
}

// SkipVersion marks the given version to be skipped (no more popups for it).
func (a *App) SkipVersion(ver string) error {
	return a.cfg.SaveSkipVersion(ver)
}

// GetSkipVersion returns the version the user chose to skip.
func (a *App) GetSkipVersion() string {
	return a.cfg.GetSkipVersion()
}

// GetAutoStart reports whether the desktop app starts with Windows.
func (a *App) GetAutoStart() bool { return autoStartEnabled() }

// SetAutoStart enables or disables starting the desktop app with Windows.
func (a *App) SetAutoStart(enabled bool) error { return setAutoStart(enabled) }

// VersionInfo describes one installed version.
type VersionInfo struct {
	Version string `json:"version"`
	Current bool   `json:"current"`
}

// List returns installed versions of the given kind ("jdk" or "maven").
func (a *App) List(kind string) []VersionInfo {
	m := manager.NewManager(a.cfg, manager.Kind(kind))
	installed, err := m.Installed()
	if err != nil {
		return []VersionInfo{}
	}
	current, _ := m.Current()
	out := make([]VersionInfo, 0, len(installed))
	for _, v := range installed {
		out = append(out, VersionInfo{Version: v, Current: v == current})
	}
	return out
}

// Current returns the current version of the given kind ("" if none).
func (a *App) Current(kind string) string {
	m := manager.NewManager(a.cfg, manager.Kind(kind))
	cur, err := m.Current()
	if err != nil {
		return ""
	}
	return cur
}

// Search returns available remote versions matching the query.
func (a *App) Search(kind, query string) []string {
	m := manager.NewManager(a.cfg, manager.Kind(kind))
	versions, err := m.Search(a.ctx, query, 50)
	if err != nil {
		return []string{}
	}
	return versions
}

// Use switches the active version.
func (a *App) Use(kind, version string) error {
	m := manager.NewManager(a.cfg, manager.Kind(kind))
	exact, err := m.Use(version)
	if err != nil {
		return err
	}
	_ = exact
	return nil
}

// Uninstall removes a version.
func (a *App) Uninstall(kind, version string) error {
	m := manager.NewManager(a.cfg, manager.Kind(kind))
	_, err := m.Uninstall(version)
	return err
}

// Import copies an existing JDK or Maven installation into the managed root.
func (a *App) Import(kind, sourcePath, version string) (string, error) {
	m := manager.NewManager(a.cfg, manager.Kind(kind))
	return m.Import(sourcePath, version)
}

// InstallProgress is emitted to the frontend during download.
type InstallProgress struct {
	Kind    string `json:"kind"`
	Version string `json:"version"`
	Done    int64  `json:"done"`
	Total   int64  `json:"total"`
	Rate    int64  `json:"rate"`
	Status  string `json:"status"` // downloading | paused | cancelled | error
}

// InstallResult is the structured outcome of an install request; the frontend
// switches on Status instead of parsing error strings.
type InstallResult struct {
	Status  string `json:"status"` // ok | paused | cancelled | error
	Message string `json:"message"`
}

// Install downloads and installs a version. Progress is emitted via the
// "install:progress" event (throttled to ~100ms); the promise resolves with a
// structured result.
func (a *App) Install(kind, version string) InstallResult {
	a.mu.Lock()
	if a.busy {
		a.mu.Unlock()
		return InstallResult{Status: "error", Message: "另一个下载正在进行中"}
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.cancel = cancel
	a.busy = true
	a.busyKey = kind + ":" + version
	a.stopMode = ""
	a.lastEmit = time.Time{}
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.busy = false
		a.cancel = nil
		a.currentCache = ""
		a.mu.Unlock()
	}()

	m := manager.NewManager(a.cfg, manager.Kind(kind))
	art, err := m.Resolve(ctx, version)
	if err != nil {
		return InstallResult{Status: "error", Message: err.Error()}
	}
	a.mu.Lock()
	a.currentCache = m.CacheFile(art)
	a.mu.Unlock()

	_, err = m.InstallWithProgress(ctx, version, func(done, total, rate int64) {
		emit := false
		a.mu.Lock()
		if a.lastEmit.IsZero() || time.Since(a.lastEmit) >= 100*time.Millisecond || done >= total {
			a.lastEmit = time.Now()
			emit = true
		}
		a.mu.Unlock()
		if emit {
			runtime.EventsEmit(a.ctx, "install:progress", InstallProgress{
				Kind:    kind,
				Version: version,
				Done:    done,
				Total:   total,
				Rate:    rate,
				Status:  "downloading",
			})
		}
	})
	if err != nil {
		if ctx.Err() == context.Canceled {
			a.mu.Lock()
			status := "cancelled"
			if a.stopMode == "pause" {
				status = "paused"
			}
			a.mu.Unlock()
			runtime.EventsEmit(a.ctx, "install:progress", InstallProgress{
				Kind:    kind,
				Version: version,
				Status:  status,
			})
			return InstallResult{Status: status, Message: "已取消"}
		}
		runtime.EventsEmit(a.ctx, "install:progress", InstallProgress{
			Kind:    kind,
			Version: version,
			Status:  "error",
		})
		return InstallResult{Status: "error", Message: err.Error()}
	}
	return InstallResult{Status: "ok", Message: "安装完成"}
}

// PauseInstall pauses the current download, keeping the partial file for resume.
func (a *App) PauseInstall() {
	a.mu.Lock()
	cancel := a.cancel
	if cancel != nil {
		a.stopMode = "pause"
	}
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// CancelInstall cancels the current download and removes the partial cache file.
func (a *App) CancelInstall() {
	a.mu.Lock()
	cancel := a.cancel
	cache := a.currentCache
	if cancel != nil {
		a.stopMode = "cancel"
	}
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if cache != "" {
		os.Remove(cache)
	}
}
