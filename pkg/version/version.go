package version

import (
	"context"
	"regexp"
	"strings"
)

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
