package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"

	"github.com/getlantern/systray"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// setupTray launches the system tray in a background goroutine.
// systray.Run() initialises the native message loop on a dedicated thread,
// then calls onReady (on the same thread) to set up the icon and menu.
func setupTray() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("tray: setup failed (recovered): %v", r)
			}
		}()
		trayIconBytes = generateTrayIcon()
		systray.Run(trayOnReady, trayOnExit)
	}()
}

// trayOnReady runs on the systray thread after the native loop starts.
// It sets up the icon, title, and menu items.
func trayOnReady() {
	systray.SetIcon(trayIconBytes)
	systray.SetTitle("jm")
	systray.SetTooltip("jm - JDK & Maven 版本管理")

	mShow := systray.AddMenuItem("显示窗口", "显示主窗口")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "退出 jm")

	// Block here to process menu events. This goroutine is already locked
	// to its OS thread by systray.Run(), so Wails calls are safe.
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

// trayOnExit is called when systray.Quit() is invoked.
func trayOnExit() {
	log.Println("tray: exited")
}

// cleanupTray shuts down the tray icon when the app exits.
func cleanupTray() {
	defer func() { recover() }()
	systray.Quit()
}

// generateTrayIcon builds a 16×16 blue square PNG at startup.
// Replace with a real .ico/.png for production use.
func generateTrayIcon() []byte {
	const size = 16
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	c := color.RGBA{R: 59, G: 130, B: 246, A: 255}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		log.Printf("tray: failed to encode icon: %v", err)
		return nil
	}
	fmt.Printf("tray: generated %d-byte icon\n", buf.Len())
	return buf.Bytes()
}
