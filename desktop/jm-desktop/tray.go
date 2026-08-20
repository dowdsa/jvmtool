package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"

	"github.com/nicehash/systray"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// trayIconBytes holds the generated tray icon (a small blue square).
var trayIconBytes []byte

// setupTray initialises the system tray in a background goroutine.
func setupTray() {
	trayIconBytes = generateTrayIcon()
	go initTray()
}

// initTray configures the tray icon and menu, and processes click events.
// Called from a goroutine; nicehash/systray runs its event loop on a
// separate OS thread on Windows so it does not block the Wails main loop.
func initTray() {
	systray.SetIcon(trayIconBytes)
	systray.SetTitle("jm")
	systray.SetTooltip("jm - JDK & Maven 版本管理")

	mShow := systray.AddMenuItem("显示窗口", "显示主窗口")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "退出 jm")

	for {
		select {
		case <-mShow.ClickedCh:
			wailsruntime.WindowShow(app.ctx)
			wailsruntime.WindowSetAlwaysOnTop(app.ctx, true)
			wailsruntime.WindowSetAlwaysOnTop(app.ctx, false)
		case <-mQuit.ClickedCh:
			systray.Quit()
			wailsruntime.Quit(app.ctx)
			return
		}
	}
}

// cleanupTray shuts down the tray icon when the app exits.
func cleanupTray() {
	systray.Quit()
}

// generateTrayIcon builds a simple 16×16 blue square PNG at startup.
// This avoids embedding an external icon file. Replace with a real
// .ico/.png for production use.
func generateTrayIcon() []byte {
	const size = 16
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	c := color.RGBA{R: 59, G: 130, B: 246, A: 255}
	for i := range img.Pix {
		if i%4 == 0 {
			img.Pix[i] = c.R
			img.Pix[i+1] = c.G
			img.Pix[i+2] = c.B
			img.Pix[i+3] = c.A
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
