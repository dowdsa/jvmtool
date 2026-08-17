package manager

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"jm/pkg/version"
)

// Import copies an existing JDK or Maven installation into JVMTOOL_HOME.
func (m *Manager) Import(sourcePath, versionArg string) (string, error) {
	source, err := filepath.Abs(filepath.Clean(sourcePath))
	if err != nil {
		return "", err
	}
	info, err := os.Stat(source)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("import source is not a directory: %s", source)
	}
	ver := strings.TrimSpace(versionArg)
	if ver == "" {
		ver = filepath.Base(source)
		if m.Kind == KindJDK {
			ver = version.NormalizeJDKVersion(ver)
		}
		if m.Kind == KindMaven {
			ver = strings.TrimPrefix(strings.TrimPrefix(ver, "apache-maven-"), "maven-")
		}
	}
	if ver == "" || ver == "." || ver == ".." || strings.ContainsAny(ver, `/\\`) {
		return "", fmt.Errorf("invalid imported version %q", ver)
	}
	if err := validateImportLayout(m.Kind, source); err != nil {
		return "", err
	}
	release, err := m.lock(context.Background())
	if err != nil {
		return "", err
	}
	defer release()
	target := m.installPath(ver)
	if _, err := os.Stat(target); err == nil {
		return "", fmt.Errorf("%s %s is already installed", m.Kind, ver)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if sameOrChildPath(source, target) || sameOrChildPath(target, source) {
		return "", fmt.Errorf("import source and target overlap")
	}
	if err := copyDirectory(source, target); err != nil {
		_ = os.RemoveAll(target)
		return "", fmt.Errorf("import failed: %w", err)
	}
	return ver, nil
}

func validateImportLayout(kind Kind, root string) error {
	bin := filepath.Join(root, "bin")
	names := []string{"java", "java.exe"}
	if kind == KindMaven {
		names = []string{"mvn", "mvn.cmd", "mvn.bat"}
	}
	for _, name := range names {
		if info, err := os.Stat(filepath.Join(bin, name)); err == nil && !info.IsDir() {
			return nil
		}
	}
	if kind == KindJDK {
		return fmt.Errorf("not a valid JDK directory: missing bin/java")
	}
	return fmt.Errorf("not a valid Maven directory: missing bin/mvn")
}

func sameOrChildPath(path, parent string) bool {
	rel, err := filepath.Rel(parent, path)
	return err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))))
}

func copyDirectory(source, target string) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(target, info.Mode().Perm()); err != nil {
		return err
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		src, dst := filepath.Join(source, entry.Name()), filepath.Join(target, entry.Name())
		entryInfo, err := os.Lstat(src)
		if err != nil {
			return err
		}
		if entryInfo.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(src)
			if err != nil {
				return err
			}
			if err := os.Symlink(link, dst); err != nil {
				return err
			}
			continue
		}
		if entryInfo.IsDir() {
			if err := copyDirectory(src, dst); err != nil {
				return err
			}
			continue
		}
		if !entryInfo.Mode().IsRegular() {
			return fmt.Errorf("unsupported file type: %s", src)
		}
		if err := copyFile(src, dst, entryInfo.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(source, target string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
