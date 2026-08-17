//go:build !windows

package config

import "os"

func proxyEnv(key string) string { return os.Getenv(key) }

func systemProxy() string { return "" }
