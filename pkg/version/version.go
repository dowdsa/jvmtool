package version

import (
	"context"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Version is the current tool version. Override at build time with:
//
//	go build -ldflags "-X jm/pkg/version.Version=v0.3.0"
var Version = "dev"

// Artifact is a downloadable distribution.
type Artifact struct {
	Version  string
	Filename string
	URL      string
	// Mirrors are alternative (typically faster) download URLs, tried in order.
	Mirrors []string
	Size    int64
	SHA256  string
	SHA512  string
}

// Source is a remote provider of tool distributions.
type Source interface {
	// List returns available versions (newest first), filtered by query.
	List(ctx context.Context, query string, limit int) ([]string, error)
	// Resolve turns a (possibly partial) version into a concrete Artifact.
	Resolve(ctx context.Context, version string) (*Artifact, error)
}

var plusRe = regexp.MustCompile(`^\+`)

// NormalizeJDKVersion converts a user input like "17", "17.0.13",
// "jdk-17.0.13+11" into an Adoptium style version string.
func NormalizeJDKVersion(input string) string {
	v := strings.TrimSpace(input)
	v = strings.TrimPrefix(v, "jdk-")
	v = strings.TrimPrefix(v, "jdk")
	return v
}

func IsPreRelease(v string) bool {
	return strings.Contains(v, "-alpha") ||
		strings.Contains(v, "-beta") ||
		strings.Contains(v, "-milestone") ||
		strings.Contains(v, "-rc")
}

// sortVersionsDesc sorts dotted version strings (e.g. "3.9.11", "17.0.13+11")
// in descending order, comparing numeric segments and ignoring pre-release /
// build suffixes.
func sortVersionsDesc(list []string) {
	sort.Slice(list, func(i, j int) bool {
		return compareVersions(list[i], list[j]) > 0
	})
}

// compareVersions compares two dotted version strings segment by segment.
// Returns -1 if a < b, 0 if equal, 1 if a > b.
func compareVersions(a, b string) int {
	ap := versionParts(a)
	bp := versionParts(b)
	n := len(ap)
	if len(bp) > n {
		n = len(bp)
	}
	for i := 0; i < n; i++ {
		var x, y int
		if i < len(ap) {
			x = ap[i]
		}
		if i < len(bp) {
			y = bp[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

// versionParts splits a version into numeric segments, stripping pre-release
// ("-rc1") and build ("+11") suffixes from each segment. Non-numeric segments
// are skipped.
func versionParts(v string) []int {
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		if i := strings.IndexAny(p, "-+"); i >= 0 {
			p = p[:i]
		}
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			continue
		}
		out = append(out, n)
	}
	return out
}
