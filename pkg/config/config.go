package config

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

const (
	envRoot  = "JVMTOOL_HOME"
	envProxy = "JVMTOOL_PROXY"
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

// ProxyURL returns the proxy URL to use for outbound requests, or nil to use
// a direct connection. Resolution order:
//  1. JVMTOOL_PROXY (tool-specific)
//  2. HTTPS_PROXY / HTTP_PROXY / ALL_PROXY (standard env vars)
//
// NOTE: reads env vars directly (not http.ProxyFromEnvironment) so changes
// are picked up immediately, which matters for long-running GUI apps.
func ProxyURL() *url.URL {
	if p := os.Getenv(envProxy); p != "" {
		if u, err := url.Parse(p); err == nil {
			return u
		}
	}
	for _, key := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"} {
		if p := os.Getenv(key); p != "" {
			if u, err := url.Parse(p); err == nil {
				return u
			}
		}
	}
	return nil
}

// HTTPClient returns an *http.Client configured with the proxy (if any).
func HTTPClient() *http.Client {
	proxy := ProxyURL()
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxy),
	}
	if proxy == nil {
		transport.Proxy = nil
	}
	return &http.Client{
		Transport: transport,
		Timeout:   0,
	}
}

// HTTPClientWithTimeout returns a client with the given timeout.
func HTTPClientWithTimeout(timeout time.Duration) *http.Client {
	c := HTTPClient()
	c.Timeout = timeout
	return c
}
