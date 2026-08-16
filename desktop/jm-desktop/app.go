package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"jm/pkg/config"
	"jm/pkg/manager"
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
	return m.Uninstall(version)
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

// Install downloads and installs a version. Progress is emitted via the
// "install:progress" event; the promise resolves when done or fails.
func (a *App) Install(kind, version string) error {
	a.mu.Lock()
	if a.busy {
		a.mu.Unlock()
		return fmt.Errorf("另一个下载正在进行中")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.cancel = cancel
	a.busy = true
	a.busyKey = kind + ":" + version
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
		return err
	}
	a.mu.Lock()
	a.currentCache = m.CacheFile(art)
	a.mu.Unlock()

	_, err = m.InstallWithProgress(ctx, version, func(done, total, rate int64) {
		runtime.EventsEmit(a.ctx, "install:progress", InstallProgress{
			Kind:    kind,
			Version: version,
			Done:    done,
			Total:   total,
			Rate:    rate,
			Status:  "downloading",
		})
	})
	if err != nil {
		if ctx.Err() == context.Canceled {
			runtime.EventsEmit(a.ctx, "install:progress", InstallProgress{
				Kind:    kind,
				Version: version,
				Status:  "cancelled",
			})
			return fmt.Errorf("已取消")
		}
		runtime.EventsEmit(a.ctx, "install:progress", InstallProgress{
			Kind:    kind,
			Version: version,
			Status:  "error",
		})
		return err
	}
	return nil
}

// PauseInstall pauses the current download, keeping the partial file for resume.
func (a *App) PauseInstall() {
	a.mu.Lock()
	cancel := a.cancel
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
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if cache != "" {
		os.Remove(cache)
	}
}
