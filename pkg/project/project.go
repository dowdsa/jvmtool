package project

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

const filename = ".jvmtoolrc"

// Config holds the JDK and Maven version requirements from a .jvmtoolrc file.
type Config struct {
	JDK   string // e.g. "17" or "17.0.13"
	Maven string // e.g. "3.9.11"
	Dir   string // directory containing the .jvmtoolrc
}

// Find walks from dir upward to the filesystem root looking for a .jvmtoolrc
// file. Returns nil if not found.
func Find(dir string) *Config {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil
	}
	for {
		path := filepath.Join(dir, filename)
		if cfg := readRC(path); cfg != nil {
			cfg.Dir = dir
			return cfg
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil
}

// FindFromCwd finds .jvmtoolrc starting from the current working directory.
func FindFromCwd() *Config {
	dir, err := os.Getwd()
	if err != nil {
		return nil
	}
	return Find(dir)
}

func readRC(path string) *Config {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	cfg := &Config{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(strings.ToLower(key)) {
		case "jdk":
			cfg.JDK = strings.TrimSpace(value)
		case "maven":
			cfg.Maven = strings.TrimSpace(value)
		}
	}
	if cfg.JDK == "" && cfg.Maven == "" {
		return nil
	}
	return cfg
}
