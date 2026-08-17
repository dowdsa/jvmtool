package config

import (
	"net"
	"net/url"
	"strings"
)

func shouldBypassProxy(target *url.URL) bool {
	raw := proxyEnv("NO_PROXY")
	if raw == "" {
		raw = proxyEnv("no_proxy")
	}
	if raw == "" {
		return false
	}
	host := target.Hostname()
	port := target.Port()
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "*" {
			return true
		}
		if item == "" {
			continue
		}
		if itemHost, itemPort, err := net.SplitHostPort(item); err == nil {
			if itemPort != port {
				continue
			}
			item = itemHost
		}
		if hostMatchesNoProxy(host, item) {
			return true
		}
	}
	return false
}

func hostMatchesNoProxy(host, item string) bool {
	if hostPort, _, err := net.SplitHostPort(item); err == nil {
		item = hostPort
	}
	item = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(item)), ".")
	host = strings.ToLower(host)
	if ip := net.ParseIP(host); ip != nil {
		if _, network, err := net.ParseCIDR(item); err == nil {
			return network.Contains(ip)
		}
	}
	return host == item || strings.HasSuffix(host, "."+item)
}
