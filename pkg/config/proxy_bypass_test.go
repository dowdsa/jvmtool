package config

import (
	"net/url"
	"testing"
)

func TestShouldBypassProxy(t *testing.T) {
	t.Setenv("NO_PROXY", "localhost,.internal.example,10.0.0.0/8,example.com:8443")
	cases := []struct {
		name string
		url  string
		want bool
	}{
		{"localhost", "http://localhost/api", true},
		{"domain suffix", "https://api.internal.example/v1", true},
		{"cidr", "http://10.12.0.4/file", true},
		{"port match", "https://example.com:8443/", true},
		{"port mismatch", "https://example.com:9443/", false},
		{"unmatched", "https://public.example/", false},
	}
	for _, tc := range cases {
		u, err := url.Parse(tc.url)
		if err != nil {
			t.Fatal(err)
		}
		if got := shouldBypassProxy(u); got != tc.want {
			t.Errorf("shouldBypassProxy(%s) = %v, want %v", tc.url, got, tc.want)
		}
	}
}
