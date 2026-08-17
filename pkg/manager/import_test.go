package manager

import (
	"os"
	"path/filepath"
	"testing"

	"jm/pkg/config"
)

func TestImportJDK(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(t.TempDir(), "jdk-21.0.1")
	if err := os.MkdirAll(filepath.Join(source, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "bin", "java"), []byte("fake java"), 0o755); err != nil {
		t.Fatal(err)
	}

	m := NewManager(&config.Config{Root: root}, KindJDK)
	got, err := m.Import(source, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "21.0.1" {
		t.Fatalf("Import version = %q, want %q", got, "21.0.1")
	}
	if _, err := os.Stat(filepath.Join(root, "jdk", "21.0.1", "bin", "java")); err != nil {
		t.Fatalf("imported java not found: %v", err)
	}
	if _, err := os.Stat(filepath.Join(source, "bin", "java")); err != nil {
		t.Fatalf("source was modified: %v", err)
	}
}

func TestImportRejectsInvalidLayout(t *testing.T) {
	m := NewManager(&config.Config{Root: t.TempDir()}, KindMaven)
	if _, err := m.Import(t.TempDir(), "3.9.11"); err == nil {
		t.Fatal("Import accepted a directory without bin/mvn")
	}
}
