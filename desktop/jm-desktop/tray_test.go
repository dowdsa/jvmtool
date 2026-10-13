package main

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestPngToICO verifies the ICO container produced by pngToICO is well-formed:
// getlantern/systray loads the tray icon on Windows via LoadImage, which only
// accepts .ico files — a malformed header would bring back the blank-icon bug.
func TestPngToICO(t *testing.T) {
	pngPayload := []byte{0x89, 'P', 'N', 'G', 0x01, 0x02, 0x03}
	got := pngToICO(pngPayload, 64)

	if len(got) != 22+len(pngPayload) {
		t.Fatalf("total size = %d, want %d", len(got), 22+len(pngPayload))
	}
	if reserved := binary.LittleEndian.Uint16(got[0:2]); reserved != 0 {
		t.Errorf("reserved = %d, want 0", reserved)
	}
	if typ := binary.LittleEndian.Uint16(got[2:4]); typ != 1 {
		t.Errorf("type = %d, want 1 (icon)", typ)
	}
	if count := binary.LittleEndian.Uint16(got[4:6]); count != 1 {
		t.Errorf("entry count = %d, want 1", count)
	}
	if got[6] != 64 || got[7] != 64 {
		t.Errorf("width/height = %d/%d, want 64/64", got[6], got[7])
	}
	if planes := binary.LittleEndian.Uint16(got[10:12]); planes != 1 {
		t.Errorf("planes = %d, want 1", planes)
	}
	if bpp := binary.LittleEndian.Uint16(got[12:14]); bpp != 32 {
		t.Errorf("bpp = %d, want 32", bpp)
	}
	if size := binary.LittleEndian.Uint32(got[14:18]); size != uint32(len(pngPayload)) {
		t.Errorf("bytesInRes = %d, want %d", size, len(pngPayload))
	}
	if offset := binary.LittleEndian.Uint32(got[18:22]); offset != 22 {
		t.Errorf("imageOffset = %d, want 22", offset)
	}
	if !bytes.Equal(got[22:], pngPayload) {
		t.Error("PNG payload not preserved verbatim")
	}
}

// TestGenerateTrayIconIsICO ensures the generated tray icon keeps its ICO
// wrapper (regression guard for the blank Windows tray icon).
func TestGenerateTrayIconIsICO(t *testing.T) {
	ico := generateTrayIcon()
	if len(ico) < 22 {
		t.Fatalf("icon too small: %d bytes", len(ico))
	}
	if typ := binary.LittleEndian.Uint16(ico[2:4]); typ != 1 {
		t.Fatalf("type = %d, want 1 (icon)", typ)
	}
	if !bytes.Equal(ico[22:27], []byte{0x89, 'P', 'N', 'G', '\r'}) {
		t.Error("embedded entry does not start with PNG signature")
	}
}
