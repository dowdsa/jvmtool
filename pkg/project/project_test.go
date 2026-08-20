package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindFromDir(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	rc := filepath.Join(root, ".jvmtoolrc")
	if err := os.WriteFile(rc, []byte("jdk=17\nmaven=3.9.11\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Should find from subdirectory.
	cfg := Find(sub)
	if cfg == nil {
		t.Fatal("expected to find .jvmtoolrc")
	}
	if cfg.JDK != "17" {
		t.Errorf("JDK = %q, want 17", cfg.JDK)
	}
	if cfg.Maven != "3.9.11" {
		t.Errorf("Maven = %q, want 3.9.11", cfg.Maven)
	}
	if cfg.Dir != root {
		t.Errorf("Dir = %q, want %q", cfg.Dir, root)
	}
}

func TestFindSkipsComments(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".jvmtoolrc"), []byte("# comment\njdk=21\n\n# maven=3.8\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := Find(root)
	if cfg == nil {
		t.Fatal("expected to find .jvmtoolrc")
	}
	if cfg.JDK != "21" {
		t.Errorf("JDK = %q, want 21", cfg.JDK)
	}
	if cfg.Maven != "" {
		t.Errorf("Maven = %q, want empty (commented out)", cfg.Maven)
	}
}

func TestFindNotFound(t *testing.T) {
	root := t.TempDir()
	if cfg := Find(root); cfg != nil {
		t.Errorf("expected nil when no .jvmtoolrc exists, got %+v", cfg)
	}
}
