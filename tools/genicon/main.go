// Command genicon renders the jm desktop application icon: a paper-white
// rounded square with the ink-black serif lettermark "jm." (gray period),
// matching the desktop UI's monochrome brand language. Outputs:
//
//	desktop/jm-desktop/build/appicon.png      (1024x1024)
//	desktop/jm-desktop/build/tray-icon.png    (64x64)
//	desktop/jm-desktop/build/windows/icon.ico (16/24/32/48/64/128/256)
//
// Run from the repo root:
//
//	go run ./tools/genicon -out desktop/jm-desktop/build
package main

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const ss = 4096 // supersample resolution

// DejaVu Serif Bold (free license, redistributable) keeps the lettermark
// reproducible on machines without the font installed.
//
//go:embed fonts/DejaVuSerif-Bold.ttf
var serifTTF []byte

var (
	paper = color.RGBA{0xf6, 0xf6, 0xf3, 255}
	ink   = color.RGBA{0x10, 0x10, 0x10, 255}
	grayL = color.RGBA{0xb0, 0xb0, 0xaa, 255}
)

func inRoundRect(x, y, cx, cy, hw, hh, r float64) bool {
	dx := math.Abs(x-cx) - (hw - r)
	dy := math.Abs(y-cy) - (hh - r)
	if dx > 0 && dy > 0 {
		return dx*dx+dy*dy <= r*r
	}
	return dx <= 0 || dy <= 0
}

func inCircle(x, y, cx, cy, r float64) bool {
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= r*r
}

func renderSS() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, ss, ss))
	for py := 0; py < ss; py++ {
		y := (float64(py) + 0.5) / ss
		for px := 0; px < ss; px++ {
			x := (float64(px) + 0.5) / ss
			i := img.PixOffset(px, py)
			if inRoundRect(x, y, .5, .5, .5, .5, .21) {
				img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = paper.R, paper.G, paper.B, 255
			} else {
				img.Pix[i+3] = 0
			}
		}
	}

	f, err := opentype.Parse(serifTTF)
	if err != nil {
		panic(err)
	}
	ft, err := opentype.NewFace(f, &opentype.FaceOptions{Size: 1280, DPI: 72})
	if err != nil {
		panic(err)
	}
	text := "jm"
	m := font.MeasureString(ft, text)
	dotX := .475*float64(ss) - float64(m.Round())/2
	dotY := .585 * float64(ss)
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(ink),
		Face: ft,
		Dot:  fixed.P(int(dotX), int(dotY)),
	}
	d.DrawString(text)

	// period as a circle for exact placement and color
	for py := 0; py < ss; py++ {
		y := (float64(py) + 0.5) / ss
		for px := 0; px < ss; px++ {
			x := (float64(px) + 0.5) / ss
			if inCircle(x, y, .705, .575, .033) {
				i := img.PixOffset(px, py)
				img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = grayL.R, grayL.G, grayL.B, 255
			}
		}
	}
	return img
}

// downsample averages src into size x size (alpha-weighted).
func downsample(src *image.RGBA, size int) image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, size, size))
	step := float64(ss) / float64(size)
	for oy := 0; oy < size; oy++ {
		y0 := float64(oy) * step
		for ox := 0; ox < size; ox++ {
			x0 := float64(ox) * step
			var ra, ga, ba, aa, wa float64
			sy0, sy1 := int(math.Floor(y0)), int(math.Ceil(y0+step))
			sx0, sx1 := int(math.Floor(x0)), int(math.Ceil(x0+step))
			for sy := sy0; sy < sy1 && sy < ss; sy++ {
				fy := math.Min(float64(sy+1), y0+step) - math.Max(float64(sy), y0)
				if fy <= 0 {
					continue
				}
				for sx := sx0; sx < sx1 && sx < ss; sx++ {
					fx := math.Min(float64(sx+1), x0+step) - math.Max(float64(sx), x0)
					if fx <= 0 {
						continue
					}
					w := fx * fy
					i := sy*src.Stride + sx*4
					a := float64(src.Pix[i+3]) / 255
					ra += float64(src.Pix[i]) * a * w
					ga += float64(src.Pix[i+1]) * a * w
					ba += float64(src.Pix[i+2]) * a * w
					aa += a * w
					wa += w
				}
			}
			o := oy*out.Stride + ox*4
			if aa > 0 {
				out.Pix[o+0] = uint8(ra/aa + 0.5)
				out.Pix[o+1] = uint8(ga/aa + 0.5)
				out.Pix[o+2] = uint8(ba/aa + 0.5)
			}
			out.Pix[o+3] = uint8(aa/wa*255 + 0.5)
		}
	}
	return *out
}

func encodePNG(img image.RGBA) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, &img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// buildICO packs PNG images as PNG-compressed ICO entries (Vista+).
func buildICO(imgs []image.RGBA) []byte {
	var hdr bytes.Buffer
	binary.Write(&hdr, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(&hdr, binary.LittleEndian, uint16(1)) // type icon
	binary.Write(&hdr, binary.LittleEndian, uint16(len(imgs)))
	var body bytes.Buffer
	offset := 6 + 16*len(imgs)
	for _, im := range imgs {
		p := encodePNG(im)
		dim := im.Bounds().Dx()
		var b byte
		if dim >= 256 {
			b = 0
		} else {
			b = byte(dim)
		}
		var e bytes.Buffer
		e.WriteByte(b)
		e.WriteByte(b)
		e.WriteByte(0)
		e.WriteByte(0)                                    // reserved
		binary.Write(&e, binary.LittleEndian, uint16(1))  // planes
		binary.Write(&e, binary.LittleEndian, uint16(32)) // bpp
		binary.Write(&e, binary.LittleEndian, uint32(len(p)))
		binary.Write(&e, binary.LittleEndian, uint32(offset))
		hdr.Write(e.Bytes())
		body.Write(p)
		offset += len(p)
	}
	out := hdr.Bytes()
	return append(out, body.Bytes()...)
}

func main() {
	out := flag.String("out", "desktop/jm-desktop/build", "build directory of the desktop module")
	flag.Parse()

	fmt.Printf("rendering %dx%d supersample...\n", ss, ss)
	big := renderSS()

	if err := os.MkdirAll(filepath.Join(*out, "windows"), 0o755); err != nil {
		panic(err)
	}

	appIcon := downsample(big, 1024)
	if err := os.WriteFile(filepath.Join(*out, "appicon.png"), encodePNG(appIcon), 0o644); err != nil {
		panic(err)
	}
	tray := downsample(big, 64)
	if err := os.WriteFile(filepath.Join(*out, "tray-icon.png"), encodePNG(tray), 0o644); err != nil {
		panic(err)
	}

	sizes := []int{16, 24, 32, 48, 64, 128, 256}
	imgs := make([]image.RGBA, 0, len(sizes))
	for _, s := range sizes {
		imgs = append(imgs, downsample(big, s))
	}
	if err := os.WriteFile(filepath.Join(*out, "windows", "icon.ico"), buildICO(imgs), 0o644); err != nil {
		panic(err)
	}
	fmt.Println("wrote appicon.png, tray-icon.png, windows/icon.ico")
}
