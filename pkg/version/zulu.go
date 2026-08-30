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

const zuluAPI = "https://api.azul.com/metadata/v1/zulu/packages"

// ZuluSource implements Source backed by the Azul Zulu API.
type ZuluSource struct {
	Arch   string
	OS     string
	Client *http.Client
}

func NewZuluSource() *ZuluSource {
	return &ZuluSource{
		Arch:   zuluArch(),
		OS:     zuluOS(),
		Client: config.HTTPClient(),
	}
}

func zuluArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x64"
	case "arm64":
		return "aarch64"
	default:
		return runtime.GOARCH
	}
}

func zuluOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos"
	case "windows":
		return "windows"
	default:
		return "linux"
	}
}

type zuluPackage struct {
	Name            string `json:"name"`
	JavaVersion     []int  `json:"java_version"`
	JdkVersion      []int  `json:"jdk_version"`
	ZuluVersion     string `json:"zulu_version"`
	DownloadURL     string `json:"download_url"`
	ArchiveType     string `json:"archive_type"`
	JavaPackageType string `json:"java_package_type"`
	OS              string `json:"os"`
	Arch            string `json:"arch"`
	HWBits          string `json:"hw_bitness"`
	SHA256Hash      string `json:"sha256_hash"`
	Size            int64  `json:"size"`
	Latest          bool   `json:"latest"`
	ReleaseStatus   string `json:"release_status"`
}

func (s *ZuluSource) List(ctx context.Context, query string, limit int) ([]string, error) {
	u, _ := url.Parse(zuluAPI)
	q := url.Values{}
	q.Set("os", s.OS)
	q.Set("arch", s.Arch)
	q.Set("archive_type", zuluArchiveType())
	q.Set("java_package_type", "jdk")
	q.Set("javafx_bundled", "false")
	q.Set("release_status", "ga")
	q.Set("latest", "true")
	if query != "" {
		// API expects array format: java_version=[8] or java_version=[17]
		// For JDK 8, query is already normalized to e.g. "8", "8u431" etc.
		// Extract just the major version number for the API filter.
		majorVer := extractMajorVersion(NormalizeJDKVersion(query))
		if majorVer != "" {
			q.Set("java_version", "["+majorVer+"]")
		}
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("zulu api returned %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	var pkgs []zuluPackage
	if err := json.Unmarshal(body, &pkgs); err != nil {
		return nil, err
	}

	var out []string
	seen := make(map[string]bool)
	for _, pkg := range pkgs {
		if !pkg.Latest || pkg.ReleaseStatus != "ga" {
			continue
		}
		v := zuluVersionString(pkg)
		// When a query is given, filter results locally too
		// (e.g., query "8" should match "8.0.504")
		if query != "" && !zuluVersionMatches(v, query) {
			continue
		}
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
			if limit > 0 && len(out) >= limit {
				return out, nil
			}
		}
	}
	return out, nil
}

func (s *ZuluSource) Resolve(ctx context.Context, version string) (*Artifact, error) {
	version = NormalizeJDKVersion(version)
	u, _ := url.Parse(zuluAPI)
	q := url.Values{}
	q.Set("os", s.OS)
	q.Set("arch", s.Arch)
	q.Set("archive_type", zuluArchiveType())
	q.Set("java_package_type", "jdk")
	q.Set("javafx_bundled", "false")
	q.Set("release_status", "ga")
	// Extract major version for API filter
	majorVer := extractMajorVersion(version)
	if majorVer != "" {
		q.Set("java_version", "["+majorVer+"]")
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("zulu api returned %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}
	var pkgs []zuluPackage
	if err := json.Unmarshal(body, &pkgs); err != nil {
		return nil, err
	}
	// Find the best match: prefer Latest=true, otherwise take the first GA.
	var best *zuluPackage
	for i := range pkgs {
		p := &pkgs[i]
		if p.ReleaseStatus != "ga" {
			continue
		}
		if best == nil {
			best = p
		}
		// Prefer a package marked Latest=true; once found, keep it.
		if p.Latest {
			best = p
			break
		}
	}
	if best == nil {
		return nil, fmt.Errorf("no Zulu JDK matches %q for %s/%s", version, s.OS, s.Arch)
	}
	v := zuluVersionString(*best)
	return &Artifact{
		Version:  v,
		Filename: best.Name,
		URL:      best.DownloadURL,
		Size:     best.Size,
		SHA256:   best.SHA256Hash,
	}, nil
}

// zuluVersionString builds a version string like "17.0.13" from the API data.
func zuluVersionString(pkg zuluPackage) string {
	parts := make([]string, 0, len(pkg.JavaVersion))
	for _, n := range pkg.JavaVersion {
		parts = append(parts, strconv.Itoa(n))
	}
	return strings.Join(parts, ".")
}

// extractMajorVersion extracts the major version number from a version string.
// "17" -> "17", "17.0.13" -> "17", "8u431" -> "8", "8.0.504" -> "8".
func extractMajorVersion(version string) string {
	if strings.HasPrefix(version, "8u") {
		return "8"
	}
	parts := strings.SplitN(version, ".", 2)
	return parts[0]
}

// zuluVersionMatches checks if a Zulu version (e.g. "8.0.504") matches a query.
// Query "8" matches "8.0.504"; query "17" matches "17.0.20".
func zuluVersionMatches(version, query string) bool {
	if version == query {
		return true
	}
	majorVer := extractMajorVersion(version)
	queryMajor := extractMajorVersion(query)
	if majorVer != "" && majorVer == queryMajor {
		return true
	}
	return strings.HasPrefix(version, query+".")
}

// zuluArchiveType returns the archive format for the current platform.
func zuluArchiveType() string {
	if runtime.GOOS == "windows" {
		return "zip"
	}
	return "tar.gz"
}
