package env

import (
	"strings"
	"testing"
)

func TestShellQuote(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"/home/user/.jvmtool", "'/home/user/.jvmtool'"},
		{"/tmp/test'path", "'/tmp/test'\\''path'"},
		{"normal-path_123", "'normal-path_123'"},
	}
	for _, c := range cases {
		if got := shellQuote(c.input); got != c.want {
			t.Errorf("shellQuote(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestBlockRejectsInjection(t *testing.T) {
	// A path with shell metacharacters should be safely quoted, not interpreted.
	malicious := `/tmp"; rm -rf / #`
	block := Block(malicious)
	// The malicious string should appear inside single quotes, neutralized.
	if !strings.Contains(block, "'"+malicious+"'") {
		t.Errorf("Block did not properly quote malicious path.\nGot: %s", block)
	}
}
