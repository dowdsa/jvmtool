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

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if err := safeExtractEntry(tmp, hdr, tr); err != nil {
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
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return err
		}
		out.Close()
		rc.Close()
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
		return os.Symlink(hdr.Linkname, target)
	default:
		return nil
	}
}
