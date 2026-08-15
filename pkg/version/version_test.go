package version

import "testing"

func TestVersionMatches(t *testing.T) {
	cases := []struct {
		rel, q string
		want   bool
	}{
		{"17.0.13+11", "17", true},
		{"17.0.13+11", "17.0.13", true},
		{"17.0.13+11", "17.0.13+11", true},
		{"26.0.2+10", "17", false},
		{"8u502-b07", "8", true},
		{"8u502-b07", "8u502", true},
		{"11.0.32+9", "11", true},
		{"11.0.32+9", "11.0.32", true},
	}
	for _, c := range cases {
		if got := versionMatches(c.rel, c.q); got != c.want {
			t.Errorf("versionMatches(%q, %q) = %v, want %v", c.rel, c.q, got, c.want)
		}
	}
}

func TestFeatureVersionOf(t *testing.T) {
	cases := map[string]int{
		"17.0.13+11": 17,
		"8u502-b07":  8,
		"11.0.32+9":  11,
		"26.0.2+10":  26,
	}
	for v, want := range cases {
		if got := featureVersionOf(v); got != want {
			t.Errorf("featureVersionOf(%q) = %d, want %d", v, got, want)
		}
	}
}

func TestNormalizeJDKVersion(t *testing.T) {
	cases := map[string]string{
		"17":            "17",
		"jdk-17.0.13+11": "17.0.13+11",
		"jdk8u502-b07":  "8u502-b07",
	}
	for in, want := range cases {
		if got := NormalizeJDKVersion(in); got != want {
			t.Errorf("NormalizeJDKVersion(%q) = %q, want %q", in, got, want)
		}
	}
}
