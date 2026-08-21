package version

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"

	"jm/pkg/config"
)

const adoptiumAPI = "https://api.adoptium.net/v3"

// SupportedDistros lists the JDK distributions that can be managed.
var SupportedDistros = []string{"temurin", "zulu"}

// NewJDKSourceForDistro returns a JDK Source for the given distribution.
// Supported: "temurin" (default), "zulu".
func NewJDKSourceForDistro(distro string) Source {
	switch strings.ToLower(distro) {
	case "zulu":
		return NewZuluSource()
	default:
		return NewJDKSource()
	}
}

// JDKSource implements Source backed by the Adoptium (Temurin) API.
type JDKSource struct {
	Arch      string
	OS        string
	ImageType string // jdk | jre
	Client    *http.Client
}

func NewJDKSource() *JDKSource {
	return &JDKSource{
		Arch:      hostArch(),
		OS:        hostOS(),
		ImageType: "jdk",
		Client:    config.HTTPClient(),
	}
}

func hostArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x64"
	case "arm64":
		return "aarch64"
	default:
		return runtime.GOARCH
	}
}

// hostOS returns the Adoptium "os" query param for the current runtime.
func hostOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "mac"
	case "windows":
		return "windows"
	default:
		return "linux"
	}
}

type releaseNamesResp struct {
	Releases []string `json:"releases"`
}

func (s *JDKSource) List(ctx context.Context, query string, limit int) ([]string, error) {
	query = NormalizeJDKVersion(query)
	var out []string
	for page := 0; ; page++ {
		u := adoptiumAPI + "/info/release_names"
		q := url.Values{}
		q.Set("architecture", s.Arch)
		q.Set("heap_size", "normal")
		q.Set("image_type", s.ImageType)
		q.Set("jvm_impl", "hotspot")
		q.Set("os", s.OS)
		q.Set("project", "jdk")
		q.Set("release_type", "ga")
		q.Set("sort_order", "DESC")
		q.Set("page", fmt.Sprintf("%d", page))
		q.Set("page_size", "20")
		q.Set("vendor", "eclipse")
		u += "?" + q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		resp, err := s.Client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("adoptium api returned %s", resp.Status)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10 MB limit
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		var rr releaseNamesResp
		if err := json.Unmarshal(body, &rr); err != nil {
			return nil, err
		}
		pageMatch := 0
		for _, v := range rr.Releases {
			norm := NormalizeJDKVersion(v)
			if query != "" && !versionMatches(norm, query) {
				continue
			}
			pageMatch++
			out = append(out, norm)
			if limit > 0 && len(out) >= limit {
				return out, nil
			}
		}
		// stop when a page has no matches for several pages
		if pageMatch == 0 && page > 3 {
			break
		}
		if len(rr.Releases) < 20 {
			break
		}
	}
	return out, nil
}

// versionMatches reports whether release matches a partial query.
// "17" matches "17.0.13+11"; "17.0.13" matches "17.0.13+11"; exact matches too.
// JDK 8 uses legacy naming "8u382-b05", handled via prefix match.
func versionMatches(release, query string) bool {
	if release == query {
		return true
	}
	if strings.HasPrefix(release, "8u") && strings.HasPrefix(release, query) {
		return true
	}
	parts := strings.Split(query, ".")
	relParts := strings.Split(release, ".")
	if len(parts) > len(relParts) {
		return false
	}
	for i, p := range parts {
		p = strings.SplitN(p, "+", 2)[0]
		rp := strings.SplitN(relParts[i], "+", 2)[0]
		if p != rp {
			return false
		}
	}
	return true
}

func (s *JDKSource) Resolve(ctx context.Context, version string) (*Artifact, error) {
	version = NormalizeJDKVersion(version)
	exact, err := s.resolveExactVersion(ctx, version)
	if err != nil {
		return nil, err
	}
	feature := featureVersionOf(exact)

	// The /assets/version endpoint has data consistency gaps for some
	// feature releases, so resolve through /assets/feature_releases instead.
	for page := 0; ; page++ {
		u := fmt.Sprintf("%s/assets/feature_releases/%d/ga", adoptiumAPI, feature)
		q := url.Values{}
		q.Set("architecture", s.Arch)
		q.Set("heap_size", "normal")
		q.Set("image_type", s.ImageType)
		q.Set("jvm_impl", "hotspot")
		q.Set("os", s.OS)
		q.Set("project", "jdk")
		q.Set("vendor", "eclipse")
		q.Set("page", fmt.Sprintf("%d", page))
		q.Set("page_size", "20")
		u += "?" + q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		resp, err := s.Client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("adoptium api returned %s for version %s", resp.Status, exact)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10 MB limit
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		var releases []struct {
			ReleaseName string `json:"release_name"`
			Binaries    []struct {
				Package struct {
					Name     string `json:"name"`
					Link     string `json:"link"`
					Size     int64  `json:"size"`
					Checksum string `json:"checksum"`
				} `json:"package"`
			} `json:"binaries"`
		}
		if err := json.Unmarshal(body, &releases); err != nil {
			return nil, err
		}
		var found *struct {
			ReleaseName string `json:"release_name"`
			Binaries    []struct {
				Package struct {
					Name     string `json:"name"`
					Link     string `json:"link"`
					Size     int64  `json:"size"`
					Checksum string `json:"checksum"`
				} `json:"package"`
			} `json:"binaries"`
		}
		for i := range releases {
			if NormalizeJDKVersion(releases[i].ReleaseName) == exact {
				found = &releases[i]
				break
			}
		}
		if found != nil {
			if len(found.Binaries) == 0 {
				return nil, fmt.Errorf("no asset for JDK version %s", exact)
			}
			pkg := found.Binaries[0].Package
			return &Artifact{
				Version:  exact,
				Filename: pkg.Name,
				URL:      pkg.Link,
				Mirrors:  jdkMirrors(feature, s, pkg.Name),
				Size:     pkg.Size,
				SHA256:   pkg.Checksum,
			}, nil
		}
		if len(releases) < 20 {
			break
		}
	}
	return nil, fmt.Errorf("no asset for JDK version %s", exact)
}

// featureVersionOf extracts the major feature version from a release name:
// "17.0.13+11" -> 17; "8u502-b07" -> 8.
func featureVersionOf(release string) int {
	s := NormalizeJDKVersion(release)
	if strings.HasPrefix(s, "8u") {
		return 8
	}
	parts := strings.SplitN(s, ".", 2)
	n, _ := strconv.Atoi(parts[0])
	if n == 0 {
		return 8
	}
	return n
}

// resolveExactVersion resolves a partial query to the newest matching release name.
func (s *JDKSource) resolveExactVersion(ctx context.Context, query string) (string, error) {
	releases, err := s.List(ctx, query, 1)
	if err != nil {
		return "", err
	}
	if len(releases) == 0 {
		return "", fmt.Errorf("no JDK version matches %q", query)
	}
	return releases[0], nil
}

// jdkMirrors builds faster mirror URLs for a JDK package, ordered by preference.
// TUNA (Tsinghua) mirrors the Adoptium release tree under /Adoptium/<feature>/jdk/<arch>/<os>/.
func jdkMirrors(feature int, s *JDKSource, filename string) []string {
	return []string{
		fmt.Sprintf("https://mirrors.tuna.tsinghua.edu.cn/Adoptium/%d/jdk/%s/%s/%s",
			feature, s.Arch, s.OS, filename),
	}
}
