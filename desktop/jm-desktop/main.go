package main

import (
	"embed"
	"os"
	"path/filepath"
	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

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
	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "jm - JDK & Maven 版本管理",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		showError("jm-desktop 启动失败", err.Error())
	}
}
