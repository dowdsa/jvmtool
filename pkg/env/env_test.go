package env

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyAndRemoveBlock(t *testing.T) {
	dir := t.TempDir()
	rc := filepath.Join(dir, ".bashrc")
	os.WriteFile(rc, []byte("export FOO=bar\n"), 0o644)

	// cannot apply without overriding RCFile; test RemoveBlock directly
	rc2 := filepath.Join(dir, ".bashrc")
	block := Block("/tmp/jmtest")
	os.WriteFile(rc2, []byte("export FOO=bar\n"+block+"export BAZ=qux\n"), 0o644)

	if !HasBlock(rc2) {
		t.Fatal("HasBlock should detect block")
	}
	changed, err := RemoveBlock(rc2)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("RemoveBlock should report changed")
	}
	if HasBlock(rc2) {
		t.Fatal("block should be removed")
	}
	content, _ := os.ReadFile(rc2)
	text := string(content)
	if !strings.Contains(text, "export FOO=bar") || !strings.Contains(text, "export BAZ=qux") {
		t.Fatalf("surrounding lines should be preserved: %q", text)
	}

	// idempotent: second call no change
	changed, err = RemoveBlock(rc2)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("second RemoveBlock should not change")
	}
}

func TestBlockContainsKeys(t *testing.T) {
	block := Block("/tmp/jmtest")
	for _, key := range []string{"JVMTOOL_HOME", "JAVA_HOME", "M2_HOME", "MAVEN_HOME"} {
		if !strings.Contains(block, key) {
			t.Fatalf("block missing %s", key)
		}
	}
}

func TestFishRCFileAndBlock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/usr/bin/fish")

	rc := RCFile()
	want := filepath.Join(home, ".config", "fish", "config.fish")
	if rc != want {
		t.Fatalf("RCFile() = %q, want %q", rc, want)
	}

	if err := ApplyBlock("/tmp/jmtest"); err != nil {
		t.Fatalf("ApplyBlock: %v", err)
	}
	content, err := os.ReadFile(rc)
	if err != nil {
		t.Fatalf("config.fish should be created with parents: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "set -gx JVMTOOL_HOME") {
		t.Fatalf("fish block should use set -gx: %q", text)
	}
	if strings.Contains(text, "export ") {
		t.Fatalf("fish block must not contain bash export syntax: %q", text)
	}

	changed, err := RemoveBlock(rc)
	if err != nil || !changed {
		t.Fatalf("RemoveBlock fish block: changed=%v err=%v", changed, err)
	}
	if HasBlock(rc) {
		t.Fatal("fish block should be removed")
	}
}

func TestBlockForShell(t *testing.T) {
	t.Setenv("SHELL", "/usr/bin/zsh")
	if got := BlockFor("/root"); !strings.Contains(got, "export JVMTOOL_HOME") {
		t.Fatalf("zsh should get POSIX block, got %q", got)
	}
	t.Setenv("SHELL", "/usr/local/bin/fish")
	if got := BlockFor("/root"); !strings.Contains(got, "set -gx PATH") {
		t.Fatalf("fish should get fish block, got %q", got)
	}
}
