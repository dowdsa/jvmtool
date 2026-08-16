package manager

import (
	"testing"
)

func TestMatchInstalled(t *testing.T) {
	installed := []string{"8u502-b08", "17.0.13+11", "21.0.1+12", "3.9.11"}
	cases := []struct{ arg, want string }{
		{"17", "17.0.13+11"},
		{"17.0.13", "17.0.13+11"},
		{"17.0.13+11", "17.0.13+11"},
		{"21.0", "21.0.1+12"},
		{"3.9", "3.9.11"},
		{"8", "8u502-b08"},
		{"9", ""},
		{"11", ""},
	}
	for _, c := range cases {
		if got := matchInstalled(installed, c.arg); got != c.want {
			t.Errorf("matchInstalled(%v, %q) = %q, want %q", installed, c.arg, got, c.want)
		}
	}
}
