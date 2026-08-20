package main

import (
	"context"
	"embed"
	"os"
	"path/filepath"
	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

// app is the global application instance, accessible from tray.go.
var app *App

// showError 记录错误日志并（在 Windows 上）弹窗提示，
// 避免 GUI 程序无控制台导致错误不可见。
func showError(title, msg string) {
	logPath := filepath.Join(os.TempDir(), "jm-desktop.log")
	if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		f.WriteString(title + ": " + msg + "\n")
		f.Close()
	}
	if runtime.GOOS == "windows" {
		showNativeError(title, msg)
	}
}

func main() {
	app = NewApp()

	err := wails.Run(&options.App{
		Title:  "jm - JDK & Maven 版本管理",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)
			setupTray()
		},
		OnBeforeClose: func(ctx *wailsruntime.Context) bool {
			switch app.cfg.GetCloseAction() {
			case "tray":
				wailsruntime.WindowHide(app.ctx)
				return true
			case "quit":
				return false // allow close
			default: // "ask" — show dialog
				wailsruntime.EventsEmit(app.ctx, "close:confirm")
				return true
			}
		},
		OnShutdown: func(ctx *wailsruntime.Context) {
			cleanupTray()
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		showError("jm-desktop 启动失败", err.Error())
	}
}
