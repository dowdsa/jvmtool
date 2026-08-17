package update

import "testing"

func TestIsNewer(t *testing.T) {
	cases := []struct {
		current string
		latest  string
		want    bool
	}{
		{"0.3.3", "0.4.0", true},
		{"v0.3.3", "v0.4.0", true},
		{"0.4.0", "0.3.3", false},
		{"0.4.0", "0.4.0", false},
		{"0.4.0", "0.4.0-rc1", false},
	}
	for _, tc := range cases {
		if got := IsNewer(tc.current, tc.latest); got != tc.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", tc.current, tc.latest, got, tc.want)
		}
	}
}
