package config

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveProxyValidation(t *testing.T) {
	c := &Config{Root: filepath.Join(t.TempDir(), "root")}
	if err := c.SaveProxy(""); err != nil {
		t.Fatalf("clearing proxy should work: %v", err)
	}
	for _, ok := range []string{
		"http://127.0.0.1:7890",
		"https://proxy.example:8443",
		"socks5://user:pass@127.0.0.1:1080",
	} {
		if err := c.SaveProxy(ok); err != nil {
			t.Errorf("SaveProxy(%q) should be valid: %v", ok, err)
		}
	}
	for _, bad := range []string{"ftp://x", "not a url", "http://"} {
		if err := c.SaveProxy(bad); err == nil {
			t.Errorf("SaveProxy(%q) should be rejected", bad)
		}
	}
	data, err := os.ReadFile(c.SettingsPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("settings file should exist after save")
	}
}

func TestSocks5ClientDialer(t *testing.T) {
	// With a persisted socks5 proxy, HTTPClient must bypass http.ProxyURL and
	// install the custom DialContext.
	c := &Config{Root: filepath.Join(t.TempDir(), "root")}
	if err := c.SaveProxy("socks5://127.0.0.1:1080"); err != nil {
		t.Fatal(err)
	}
	cl := HTTPClient()
	tr, ok := cl.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport")
	}
	if tr.Proxy != nil {
		t.Fatal("socks5 should disable http.ProxyURL")
	}
	if tr.DialContext == nil {
		t.Fatal("socks5 should set DialContext")
	}
}
