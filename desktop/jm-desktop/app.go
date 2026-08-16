package main

import (
	"context"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"jm/pkg/config"
	"jm/pkg/manager"
)

// App struct
type App struct {
	ctx context.Context
	cfg *config.Config
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
}

// Root returns the JVMTOOL_HOME root directory.
func (a *App) Root() string {
	return a.cfg.Root
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
}

// Install downloads and installs a version. Progress is emitted via the
// "install:progress" event; the promise resolves when done or fails.
func (a *App) Install(kind, version string) error {
	m := manager.NewManager(a.cfg, manager.Kind(kind))
	_, err := m.InstallWithProgress(a.ctx, version, func(done, total int64) {
		runtime.EventsEmit(a.ctx, "install:progress", InstallProgress{
			Kind:    kind,
			Version: version,
			Done:    done,
			Total:   total,
		})
	})
	return err
}
