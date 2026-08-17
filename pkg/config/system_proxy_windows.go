//go:build windows

package config

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	userEnvironmentKey  = `Environment`
	internetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`
)

// proxyEnv also reads the user environment registry so a long-running desktop
// process sees proxy variables added after it was started.
func proxyEnv(key string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	keyHandle, err := registry.OpenKey(registry.CURRENT_USER, userEnvironmentKey, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer keyHandle.Close()
	value, _, err := keyHandle.GetStringValue(key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

// systemProxy reads the static proxy configured in Windows Internet Settings.
// PAC scripts require per-URL evaluation and are intentionally left to the
// environment-variable or manual-proxy paths until a WinINet resolver is added.
func systemProxy() string {
	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()
	enabled, _, err := key.GetIntegerValue("ProxyEnable")
	if err != nil || enabled == 0 {
		return ""
	}
	server, _, err := key.GetStringValue("ProxyServer")
	if err != nil {
		return ""
	}
	server = strings.TrimSpace(server)
	if server == "" {
		return ""
	}
	for _, part := range strings.Split(server, ";") {
		pair := strings.SplitN(part, "=", 2)
		if len(pair) == 2 && strings.EqualFold(strings.TrimSpace(pair[0]), "https") {
			return normalizeProxyURL(pair[1])
		}
	}
	for _, part := range strings.Split(server, ";") {
		pair := strings.SplitN(part, "=", 2)
		if len(pair) == 2 && strings.EqualFold(strings.TrimSpace(pair[0]), "http") {
			return normalizeProxyURL(pair[1])
		}
	}
	if strings.Contains(server, "=") {
		for _, part := range strings.Split(server, ";") {
			pair := strings.SplitN(part, "=", 2)
			if len(pair) == 2 && strings.EqualFold(strings.TrimSpace(pair[0]), "socks") {
				value := strings.TrimSpace(pair[1])
				if value != "" && !strings.Contains(value, "://") {
					return "socks5://" + value
				}
				return value
			}
		}
	}
	return normalizeProxyURL(server)
}

func normalizeProxyURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !strings.Contains(value, "://") {
		value = "http://" + value
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "socks5://") {
		return value
	}
	return fmt.Sprintf("http://%s", value)
}
