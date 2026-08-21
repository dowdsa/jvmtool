package manager

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// maxExtractSize is the maximum total bytes allowed during archive extraction.
// This protects against zip/tar bombs (e.g. 42.zip: 42KB → 4.5PB).
const maxExtractSize int64 = 2 << 30 // 2 GB

// extractArchive extracts a .tar.gz or .zip archive into destDir, moving the
// single top-level directory into destDir under its own name.
func extractArchive(src, destDir string) error {
	if strings.HasSuffix(strings.ToLower(src), ".zip") {
		return extractZip(src, destDir)
	}
	return extractTarGz(src, destDir)
}

// extractTarGz extracts a .tar.gz archive into destDir, moving the single
// top-level directory into destDir under its own name.
func extractTarGz(src, destDir string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tmp, err := os.MkdirTemp(destDir, ".extract-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	var extractedSize int64
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		extractedSize += hdr.Size
		if extractedSize > maxExtractSize {
			return fmt.Errorf("archive exceeds maximum extraction size (%d GB)", maxExtractSize>>30)
		}
		remaining := maxExtractSize - extractedSize
		if err := safeExtractEntry(tmp, hdr, io.LimitReader(tr, remaining+1)); err != nil {
			return err
		}
	}
	return moveSingleDir(tmp, destDir)
}

// extractZip extracts a .zip archive into destDir.
func extractZip(src, destDir string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	tmp, err := os.MkdirTemp(destDir, ".extract-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	var extractedSize int64
	for _, f := range r.File {
		target := filepath.Join(tmp, filepath.Clean(f.Name))
		if !strings.HasPrefix(target, filepath.Clean(tmp)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal path in archive: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		mode := f.Mode()
		if mode == 0 {
			mode = 0o755
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
		if err != nil {
			rc.Close()
			return err
		}
		n, copyErr := io.Copy(out, rc)
		out.Close()
		rc.Close()
		if copyErr != nil {
			return copyErr
		}
		extractedSize += n
		if extractedSize > maxExtractSize {
			return fmt.Errorf("archive exceeds maximum extraction size (%d GB)", maxExtractSize>>30)
		}
	}
	return moveSingleDir(tmp, destDir)
}

// moveSingleDir moves the single top-level directory in tmp into destDir.
func moveSingleDir(tmp, destDir string) error {
	entries, err := os.ReadDir(tmp)
	if err != nil {
		return err
	}
	if len(entries) == 1 && entries[0].IsDir() {
		name := entries[0].Name()
		target := filepath.Join(destDir, name)
		if _, err := os.Stat(target); err == nil {
			return fmt.Errorf("target directory %s already exists", target)
		}
		return os.Rename(filepath.Join(tmp, name), target)
	}
	return fmt.Errorf("unexpected archive layout (wanted single top-level directory)")
}

func safeExtractEntry(base string, hdr *tar.Header, tr io.Reader) error {
	target := filepath.Join(base, filepath.Clean(hdr.Name))
	if !strings.HasPrefix(target, filepath.Clean(base)+string(os.PathSeparator)) {
		return fmt.Errorf("illegal path in archive: %s", hdr.Name)
	}
	switch hdr.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, os.FileMode(hdr.Mode)&0o755)
	case tar.TypeReg, tar.TypeRegA:
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	case tar.TypeSymlink:
		// Only allow symlinks whose resolved target stays inside the
		// extraction base. Reject absolute links, drive-relative links that
		// start with a separator ("/x" or "\x" resolve against the drive
		// root on Windows), and anything that escapes the base directory.
		link := hdr.Linkname
		if filepath.IsAbs(link) || strings.HasPrefix(link, "/") || strings.HasPrefix(link, "\\") {
			return fmt.Errorf("illegal absolute symlink target in archive: %s", link)
		}
		resolved := filepath.Clean(filepath.Join(filepath.Dir(target), link))
		if !strings.HasPrefix(resolved, filepath.Clean(base)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal symlink target in archive: %s", link)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.Symlink(link, target)
	default:
		return nil
	}
}
