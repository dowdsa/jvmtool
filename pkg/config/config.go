package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	envRoot  = "JVMTOOL_HOME"
	envProxy = "JVMTOOL_PROXY"
)

type Config struct {
	Root string
}

// Settings is persisted to <root>/config.json.
type Settings struct {
	Proxy       string `json:"proxy,omitempty"`
	SkipVersion string `json:"skip_version,omitempty"`
}

// settingsCache holds the in-memory settings so ProxyURL can read them without
// hitting disk on every request.
var (
	settingsMu sync.RWMutex
	settings   Settings
)

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

func (c *Config) JDKDir() string   { return filepath.Join(c.Root, "jdk") }
func (c *Config) MavenDir() string { return filepath.Join(c.Root, "maven") }
func (c *Config) CacheDir() string { return filepath.Join(c.Root, "cache") }

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

// SettingsPath returns the config file path.
func (c *Config) SettingsPath() string {
	return filepath.Join(c.Root, "config.json")
}

// LoadSettings reads settings from disk into the in-memory cache.
func (c *Config) LoadSettings() {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	data, err := os.ReadFile(c.SettingsPath())
	if err != nil {
		settings = Settings{}
		return
	}
	var s Settings
	if json.Unmarshal(data, &s) != nil {
		settings = Settings{}
		return
	}
	settings = s
}

// SaveProxy validates and persists the proxy setting, and updates the
// in-memory cache. Supported schemes: http, https, socks5.
func (c *Config) SaveProxy(proxy string) error {
	proxy = strings.TrimSpace(proxy)
	if proxy != "" {
		u, err := url.Parse(proxy)
		if err != nil {
			return fmt.Errorf("无效代理地址: %w", err)
		}
		scheme := strings.ToLower(u.Scheme)
		if scheme != "http" && scheme != "https" && scheme != "socks5" {
			return fmt.Errorf("不支持的代理协议 %q（仅支持 http/https/socks5）", u.Scheme)
		}
		if u.Host == "" {
			return errors.New("代理地址缺少主机名")
		}
	}
	if err := c.Ensure(); err != nil {
		return err
	}
	settingsMu.Lock()
	settings.Proxy = proxy
	s := settings
	settingsMu.Unlock()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	// 0600: config.json may embed proxy credentials
	return os.WriteFile(c.SettingsPath(), data, 0o600)
}

// GetProxy returns the configured proxy string ("" if none).
func (c *Config) GetProxy() string {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	return settings.Proxy
}

// GetSkipVersion returns the version the user chose to skip ("" if none).
func (c *Config) GetSkipVersion() string {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	return settings.SkipVersion
}

// SaveSkipVersion persists the version the user wants to skip.
func (c *Config) SaveSkipVersion(version string) error {
	if err := c.Ensure(); err != nil {
		return err
	}
	settingsMu.Lock()
	settings.SkipVersion = version
	s := settings
	settingsMu.Unlock()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.SettingsPath(), data, 0o600)
}

// ProxyURL returns the proxy URL to use for outbound requests, or nil to use
// a direct connection. Resolution order:
//  1. persisted setting (config.json)
//  2. JVMTOOL_PROXY (tool-specific env)
//  3. HTTPS_PROXY / HTTP_PROXY / ALL_PROXY (standard env vars)
//  4. Windows user environment and Internet Settings proxy
//
// NOTE: reads directly (not http.ProxyFromEnvironment) so changes are picked
// up immediately, which matters for long-running GUI apps.
func ProxyURL() *url.URL {
	if p := GetProxyString(); p != "" {
		if u, err := url.Parse(p); err == nil {
			return u
		}
	}
	for _, key := range []string{envProxy, "HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"} {
		if p := proxyEnv(key); p != "" {
			if u, err := url.Parse(p); err == nil {
				return u
			}
		}
	}
	if p := systemProxy(); p != "" {
		if u, err := url.Parse(p); err == nil {
			return u
		}
	}
	return nil
}

// GetProxyString returns the persisted proxy string.
func GetProxyString() string {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	return settings.Proxy
}

// SystemPACURL returns the configured Windows PAC URL, if any.
func SystemPACURL() string { return systemPAC() }

// Cached HTTP client to enable TCP connection reuse. Rebuilt when the
// effective proxy changes so proxy updates are picked up immediately.
var (
	clientMu     sync.Mutex
	cachedClient *http.Client
	cachedProxy  string
)

// HTTPClient returns an *http.Client configured with the proxy (if any) and
// sane connect/header timeouts. The overall request timeout stays 0 so large
// downloads are not capped; hung servers are still bounded by the dial and
// response-header timeouts.
//
// The client is cached and reused across calls so TCP connections (and TLS
// sessions) are kept alive. The cache is invalidated when the resolved proxy
// changes, so long-running processes (desktop GUI) still pick up proxy edits.
func HTTPClient() *http.Client {
	proxy := ProxyURL()
	proxyKey := ""
	if proxy != nil {
		proxyKey = proxy.String()
	}
	clientMu.Lock()
	defer clientMu.Unlock()
	if cachedClient != nil && cachedProxy == proxyKey {
		return cachedClient
	}
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ResponseHeaderTimeout: 30 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		IdleConnTimeout:       90 * time.Second,
	}
	if proxy != nil {
		if isSocks5Scheme(proxy) {
			// stdlib Transport has no socks5 proxy support; dial through it.
			transport.DialContext = socks5Dialer(proxy)
		} else {
			transport.Proxy = func(req *http.Request) (*url.URL, error) {
				if shouldBypassProxy(req.URL) {
					return nil, nil
				}
				return proxy, nil
			}
		}
	}
	cachedClient = &http.Client{Transport: transport, Timeout: 0}
	cachedProxy = proxyKey
	return cachedClient
}

// HTTPClientWithTimeout returns a client with the given timeout. It reuses
// the same Transport (connection pool) as HTTPClient but creates a separate
// client wrapper so the timeout does not affect the cached default client.
func HTTPClientWithTimeout(timeout time.Duration) *http.Client {
	base := HTTPClient()
	return &http.Client{
		Transport: base.Transport,
		Timeout:   timeout,
	}
}
