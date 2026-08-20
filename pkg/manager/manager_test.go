package manager

import (
	"os"
	"path/filepath"
	"testing"

	"jm/pkg/config"
)

func TestMatchInstalled(t *testing.T) {
	installed := []string{"8u502-b08", "17.0.13+11", "21.0.1+12", "3.9.11"}
	cases := []struct{ arg, want string }{
		{"17", "17.0.13+11"},
		{"17.0.13", "17.0.13+11"},
		{"17.0.13+11", "17.0.13+11"},
		{"21.0", "21.0.1+12"},
		{"3.9", "3.9.11"},
		{"8", "8u502-b08"},
		{"9", ""},
		{"11", ""},
		// "1" matches both 17 and 11.0.32 via prefix — should be rejected.
	}
	for _, c := range cases {
		if got := matchInstalled(installed, c.arg); got != c.want {
			t.Errorf("matchInstalled(%v, %q) = %q, want %q", installed, c.arg, got, c.want)
		}
	}
	// Ambiguous prefix: "1" matches 17 and 21 — should return empty.
	ambiguous := []string{"11.0.32+9", "17.0.13+11"}
	if got := matchInstalled(ambiguous, "1"); got != "" {
		t.Errorf("matchInstalled(%v, %q) = %q, want empty (ambiguous)", ambiguous, "1", got)
	}
}

func TestNewestRemaining(t *testing.T) {
	m := NewManager(&config.Config{Root: t.TempDir()}, KindJDK)
	installed := []string{"8u502-b08", "17.0.13+11", "21.0.1+12"}
	if got := m.newestRemaining(installed, "21.0.1+12"); got != "17.0.13+11" {
		t.Errorf("newestRemaining excluding 21 = %q, want 17.0.13+11", got)
	}
	if got := m.newestRemaining(installed, "17.0.13+11"); got != "21.0.1+12" {
		t.Errorf("newestRemaining excluding 17 = %q, want 21.0.1+12", got)
	}
	if got := m.newestRemaining([]string{"17.0.13+11"}, "17.0.13+11"); got != "" {
		t.Errorf("newestRemaining with only excluded = %q, want empty", got)
	}
}

func TestRemoveVersionCache(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{Root: root}
	if err := cfg.Ensure(); err != nil {
		t.Fatal(err)
	}
	m := NewManager(cfg, KindJDK)
	matching := filepath.Join(cfg.CacheDir(), "OpenJDK17U-jdk_x64_17.0.13_11.zip")
	other := filepath.Join(cfg.CacheDir(), "OpenJDK21U-jdk_x64_21.0.1_12.zip")
	for _, path := range []string{matching, other} {
		if err := os.WriteFile(path, []byte("partial"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.removeVersionCache("17.0.13+11"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(matching); !os.IsNotExist(err) {
		t.Fatalf("matching cache still exists: %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("unrelated cache was removed: %v", err)
	}
}
