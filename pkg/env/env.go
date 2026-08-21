package env

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const marker = "# >>> jm >>>"

// RCFile returns the shell rc file to manage based on the current shell.
func RCFile() string {
	shell := os.Getenv("SHELL")
	if strings.Contains(shell, "zsh") {
		return filepath.Join(mustHome(), ".zshrc")
	}
	return filepath.Join(mustHome(), ".bashrc")
}

// HasBlock reports whether the rc file contains the jm env block.
func HasBlock(rcFile string) bool {
	content, err := os.ReadFile(rcFile)
	if err != nil {
		return false
	}
	return strings.Contains(string(content), marker)
}

// shellQuote wraps s in single quotes for safe shell embedding.
// Single quotes in s are escaped as '\'' (end quote, escaped quote, start quote).
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// Block returns the environment block content written by install.sh.
func Block(root string) string {
	var b strings.Builder
	b.WriteString(marker + "\n")
	fmt.Fprintf(&b, "export JVMTOOL_HOME=\"${JVMTOOL_HOME:-%s}\"\n", shellQuote(root))
	b.WriteString("if [ -d \"$JVMTOOL_HOME/jdk/current\" ]; then\n")
	b.WriteString("    export JAVA_HOME=\"$JVMTOOL_HOME/jdk/current\"\n")
	b.WriteString("    export PATH=\"$JAVA_HOME/bin:$PATH\"\n")
	b.WriteString("fi\n")
	b.WriteString("if [ -d \"$JVMTOOL_HOME/maven/current\" ]; then\n")
	b.WriteString("    export M2_HOME=\"$JVMTOOL_HOME/maven/current\"\n")
	b.WriteString("    export MAVEN_HOME=\"$M2_HOME\"\n")
	b.WriteString("    export PATH=\"$M2_HOME/bin:$PATH\"\n")
	b.WriteString("fi\n")
	b.WriteString(marker + "\n")
	return b.String()
}

// RemoveBlock removes the jm env block from the rc file.
// Returns (changed, error).
func RemoveBlock(rcFile string) (bool, error) {
	content, err := os.ReadFile(rcFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	text := string(content)
	if !strings.Contains(text, marker) {
		return false, nil
	}

	lines := strings.Split(text, "\n")
	var out []string
	inBlock := false
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == marker {
			// toggle: first marker opens, second closes
			inBlock = !inBlock
			continue
		}
		if !inBlock {
			out = append(out, l)
		}
	}
	// trim trailing blank lines
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	result := strings.Join(out, "\n") + "\n"
	if result == text {
		return false, nil
	}
	return true, os.WriteFile(rcFile, []byte(result), 0o644)
}

// ApplyBlock writes (or updates) the jm env block in the rc file.
func ApplyBlock(root string) error {
	rcFile := RCFile()
	RemoveBlock(rcFile) // idempotent

	content, err := os.ReadFile(rcFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	text := strings.TrimRight(string(content), "\n")
	block := "\n" + Block(root)
	if text == "" {
		block = strings.TrimPrefix(block, "\n")
	}
	return os.WriteFile(rcFile, []byte(text+block), 0o644)
}

// CurrentValues reports the active env configuration.
func CurrentValues(root string) map[string]string {
	values := map[string]string{
		"JVMTOOL_HOME": os.Getenv("JVMTOOL_HOME"),
		"JAVA_HOME":    os.Getenv("JAVA_HOME"),
		"M2_HOME":      os.Getenv("M2_HOME"),
		"MAVEN_HOME":   os.Getenv("MAVEN_HOME"),
	}
	if values["JVMTOOL_HOME"] == "" {
		values["JVMTOOL_HOME"] = root
	}
	return values
}

func mustHome() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return h
}

// IsWindows reports whether running on Windows.
func IsWindows() bool {
	return runtime.GOOS == "windows"
}
