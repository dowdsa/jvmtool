package config

import (
	"os"
	"path/filepath"
)

const (
	envRoot = "JVMTOOL_HOME"
)

type Config struct {
	Root string
}

func Default() *Config {
	root := os.Getenv(envRoot)
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		root = filepath.Join(home, ".jvmtool")
	}
	return &Config{Root: root}
}

func (c *Config) JDKDir() string      { return filepath.Join(c.Root, "jdk") }
func (c *Config) MavenDir() string    { return filepath.Join(c.Root, "maven") }
func (c *Config) CacheDir() string    { return filepath.Join(c.Root, "cache") }

func (c *Config) JDKPath(version string) string   { return filepath.Join(c.JDKDir(), version) }
func (c *Config) MavenPath(version string) string { return filepath.Join(c.MavenDir(), version) }

func (c *Config) Ensure() error {
	for _, d := range []string{c.Root, c.JDKDir(), c.MavenDir(), c.CacheDir()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}
