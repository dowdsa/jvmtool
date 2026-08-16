package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"jm/pkg/config"
)

const (
	repoOwner = "dowdsa"
	repoName  = "jvmtool"
)

// Release describes the latest release metadata.
type Release struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// Version strips the leading "v" from a tag (e.g. "v0.3.0" -> "0.3.0").
func (r *Release) Version() string {
	return strings.TrimPrefix(r.TagName, "v")
}

// Latest fetches the latest release from GitHub.
func Latest(ctx context.Context) (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
	client := config.HTTPClientWithTimeout(15 * time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "jm-updater")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned %s", resp.Status)
	}
	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// Compare compares two dotted version strings (e.g. "0.3.0" vs "0.3.1").
// Returns -1 if a < b, 0 if equal, 1 if a > b.
func Compare(a, b string) int {
	av := parseParts(a)
	bv := parseParts(b)
	n := len(av)
	if len(bv) > n {
		n = len(bv)
	}
	for i := 0; i < n; i++ {
		var x, y int
		if i < len(av) {
			x = av[i]
		}
		if i < len(bv) {
			y = bv[i]
		}
		if x < y {
			return -1
		}
		if x > y {
			return 1
		}
	}
	return 0
}

func parseParts(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		// strip non-numeric suffix like "-rc1", "+build"
		num := p
		if i := strings.IndexAny(p, "-+"); i >= 0 {
			num = p[:i]
		}
		n, err := strconv.Atoi(num)
		if err != nil {
			n = 0
		}
		out = append(out, n)
	}
	return out
}

// IsNewer reports whether latest is a newer version than current.
func IsNewer(current, latest string) bool {
	return Compare(current, latest) < 0
}
