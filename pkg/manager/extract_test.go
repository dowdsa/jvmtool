package manager

import (
	"archive/tar"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeExtractEntrySymlinkValidation(t *testing.T) {
	base := t.TempDir()

	// an in-base relative symlink is allowed
	h := &tar.Header{Name: "sub/link", Typeflag: tar.TypeSymlink, Linkname: "target.txt", Mode: 0o755}
	if err := safeExtractEntry(base, h, nil); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "privilege") || strings.Contains(strings.ToLower(err.Error()), "privileg") {
			t.Skipf("symlink creation requires additional Windows privilege: %v", err)
		}
		t.Fatalf("in-base symlink should be allowed: %v", err)
	}
	if fi, err := os.Lstat(filepath.Join(base, "sub", "link")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected a symlink to exist: %v", err)
	}

	// escaping / absolute link targets must be rejected
	for _, l := range []string{"../escape", "../../../etc/passwd", "/etc/passwd"} {
		h := &tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: l, Mode: 0o755}
		if err := safeExtractEntry(base, h, nil); err == nil {
			t.Errorf("symlink target %q should be rejected", l)
		}
	}
}

func TestSafeExtractEntryPathTraversal(t *testing.T) {
	base := t.TempDir()
	h := &tar.Header{Name: "../evil.txt", Typeflag: tar.TypeReg, Mode: 0o644}
	if err := safeExtractEntry(base, h, nil); err == nil {
		t.Fatal("path traversal entry should be rejected")
	}
}
