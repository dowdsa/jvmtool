package version

import (
	"reflect"
	"testing"
)

func TestSortVersionsDesc(t *testing.T) {
	in := []string{"3.8.8", "3.9.11", "3.9.2", "3.9.9", "4.0.0-rc1", "3.9.11"}
	want := []string{"4.0.0-rc1", "3.9.11", "3.9.11", "3.9.9", "3.9.2", "3.8.8"}
	sortVersionsDesc(in)
	if !reflect.DeepEqual(in, want) {
		t.Fatalf("sortVersionsDesc = %v, want %v", in, want)
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"3.9.11", "3.9.9", 1},
		{"3.9", "3.9.1", -1},
		{"17.0.13+11", "17.0.13+9", 0}, // build suffix ignored
		{"4.0.0-rc1", "3.9.11", 1},
		{"v0.3.0", "v0.4.0", -1}, // v prefix stripped
	}
	for _, c := range cases {
		if got := CompareVersions(c.a, c.b); got != c.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
