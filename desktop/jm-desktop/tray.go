package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"log"

	"github.com/getlantern/systray"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// trayIconBytes holds the generated tray icon PNG data.
var trayIconBytes []byte

// setupTray launches the system tray in a background goroutine.
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
func trayOnReady() {
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
			// RequestQuit sets quitPending so the quit is not cancelled by
			// OnBeforeClose re-running in "ask" mode.
			app.RequestQuit()
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

// generateTrayIcon builds a 64×64 icon with dark background and "jm" lettermark,
// returned as ICO bytes. Windows system tray requires at least 32×32; 64×64
// ensures clarity on HiDPI.
func generateTrayIcon() []byte {
	const size = 64
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	// Dark background matching app theme (#101010)
	bg := color.RGBA{R: 16, G: 16, B: 16, A: 255}
	fg := color.RGBA{R: 245, G: 245, B: 241, A: 255}    // off-white
	accent := color.RGBA{R: 59, G: 130, B: 246, A: 255} // blue

	// Fill background
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, bg)
		}
	}

	// Draw rounded corners (radius 14)
	roundTrayCorners(img, 14)

	// Draw "j" letter
	// Vertical stem
	fillRect(img, 14, 18, 24, 44, fg)
	// Dot above
	fillRect(img, 16, 10, 22, 15, fg)
	// Bottom hook (curve left)
	fillRect(img, 10, 40, 14, 50, fg)

	// Draw "m" letter
	// Left vertical
	fillRect(img, 28, 22, 35, 50, fg)
	// Middle vertical
	fillRect(img, 38, 22, 45, 50, fg)
	// Right vertical
	fillRect(img, 48, 22, 55, 50, fg)
	// Left arch
	fillRect(img, 35, 18, 38, 26, fg)
	// Right arch
	fillRect(img, 45, 18, 48, 26, fg)

	// Blue accent bar at bottom
	fillRect(img, 10, 54, 54, 58, accent)

	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		log.Printf("tray: failed to encode icon: %v", err)
		return nil
	}
	// getlantern/systray writes the bytes to a temp file and loads it via
	// LoadImage(IMAGE_ICON|LR_LOADFROMFILE), which only reads .ico files —
	// raw PNG leaves a blank tray icon on Windows.
	return pngToICO(pngBuf.Bytes(), size)
}

// pngToICO wraps a single PNG image in a minimal ICO container. PNG-compressed
// ICO entries are supported since Windows Vista, so no re-encoding is needed.
func pngToICO(png []byte, size int) []byte {
	var out bytes.Buffer
	binary.Write(&out, binary.LittleEndian, uint16(0))  // reserved
	binary.Write(&out, binary.LittleEndian, uint16(1))  // type: icon
	binary.Write(&out, binary.LittleEndian, uint16(1))  // entry count
	out.WriteByte(byte(size))                           // width (0 means 256)
	out.WriteByte(byte(size))                           // height
	out.WriteByte(0)                                    // palette size
	out.WriteByte(0)                                    // reserved
	binary.Write(&out, binary.LittleEndian, uint16(1))  // color planes
	binary.Write(&out, binary.LittleEndian, uint16(32)) // bits per pixel
	binary.Write(&out, binary.LittleEndian, uint32(len(png)))
	binary.Write(&out, binary.LittleEndian, uint32(22)) // data offset (6+16)
	out.Write(png)
	return out.Bytes()
}

func fillRect(img *image.RGBA, x1, y1, x2, y2 int, c color.RGBA) {
	for y := y1; y < y2; y++ {
		for x := x1; x < x2; x++ {
			img.Set(x, y, c)
		}
	}
}

// roundTrayCorners clears the corners to make the icon rounded.
func roundTrayCorners(img *image.RGBA, r int) {
	size := img.Bounds().Max.X
	transparent := color.RGBA{A: 0}
	corners := [][2]int{
		{r, r},                       // top-left center
		{size - r - 1, r},            // top-right center
		{r, size - r - 1},            // bottom-left center
		{size - r - 1, size - r - 1}, // bottom-right center
	}
	for _, c := range corners {
		cx, cy := c[0], c[1]
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				// Only check pixels in this corner's quadrant
				inCorner := false
				if cx == r && cy == r && x <= r && y <= r {
					inCorner = true
				} else if cx > r && cy == r && x >= cx && y <= r {
					inCorner = true
				} else if cx == r && cy > r && x <= r && y >= cy {
					inCorner = true
				} else if cx > r && cy > r && x >= cx && y >= cy {
					inCorner = true
				}
				if inCorner {
					dx := float64(x - cx)
					dy := float64(y - cy)
					if dx*dx+dy*dy > float64(r*r) {
						img.Set(x, y, transparent)
					}
				}
			}
		}
	}
}
