package cmd

import (
	"io"
	"os"
	"strings"
	"testing"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	fn()
	w.Close()
	os.Stdout = old
	out := <-done
	r.Close()
	return out
}

func TestRootCommandRegistration(t *testing.T) {
	want := []string{"jdk", "maven", "status", "use", "clean", "cache", "env", "update", "doctor", "completion"}
	got := map[string]bool{}
	for _, c := range rootCmd.Commands() {
		got[c.Name()] = true
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("subcommand %q not registered on root", name)
		}
	}
}

func TestToolGroupSubcommands(t *testing.T) {
	want := []string{"search", "info", "install", "list", "use", "uninstall", "current", "import"}
	for _, kind := range []kindString{KindJDK, KindMaven} {
		group := newToolGroup(kind, kind)
		got := map[string]bool{}
		for _, c := range group.Commands() {
			got[c.Name()] = true
		}
		for _, name := range want {
			if !got[name] {
				t.Errorf("%s group missing subcommand %q", kind, name)
			}
		}
	}
}

func TestDistroFlagRouting(t *testing.T) {
	jdk := newToolGroup("jdk", KindJDK)
	if err := jdk.ParseFlags(nil); err != nil {
		t.Fatalf("parse empty flags: %v", err)
	}
	if v := getDistro(jdk); v != "openjdk" {
		t.Errorf("default distro = %q, want openjdk", v)
	}
	if err := jdk.ParseFlags([]string{"--distro", "zulu"}); err != nil {
		t.Fatalf("parse --distro zulu: %v", err)
	}
	if v := getDistro(jdk); v != "zulu" {
		t.Errorf("distro = %q, want zulu", v)
	}

	maven := newToolGroup("maven", KindMaven)
	if maven.PersistentFlags().Lookup("distro") != nil {
		t.Error("maven group must not carry --distro")
	}
	if v := getDistro(maven); v != "" {
		t.Errorf("getDistro(maven) = %q, want empty", v)
	}
	if v := getDistro(nil); v != "" {
		t.Errorf("getDistro(nil) = %q, want empty", v)
	}
}

func TestMavenListRejectsDistroFlag(t *testing.T) {
	oldRoot := cfg.Root
	defer func() { cfg.Root = oldRoot }()
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	defer func() { rootCmd.SilenceUsage = false; rootCmd.SilenceErrors = false }()

	rootCmd.SetArgs([]string{"maven", "list", "--distro", "zulu", "--root", t.TempDir()})
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("maven list --distro should fail with unknown flag, got %v", err)
	}
}

func TestJDKListOnEmptyRoot(t *testing.T) {
	oldRoot := cfg.Root
	defer func() { cfg.Root = oldRoot }()

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"jdk", "list", "--root", t.TempDir()})
		if err := rootCmd.Execute(); err != nil {
			t.Errorf("jdk list on empty root: %v", err)
		}
	})
	if !strings.Contains(out, "尚未安装任何 jdk 版本") {
		t.Fatalf("unexpected output: %q", out)
	}
}
