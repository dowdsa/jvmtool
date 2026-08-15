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
