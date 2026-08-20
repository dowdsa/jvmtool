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
	var out []string
	for page := 1; ; page++ {
		u, _ := url.Parse(zuluAPI)
		q := url.Values{}
		q.Set("os", s.OS)
		q.Set("arch", s.Arch)
		q.Set("archive_type", zuluArchiveType())
		q.Set("java_package_type", "jdk")
		q.Set("javafx_bundled", "false")
		q.Set("release_status", "ga")
		q.Set("page", strconv.Itoa(page))
		q.Set("page_size", "50")
		if query != "" {
			q.Set("java_version", NormalizeJDKVersion(query))
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
		body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		var pkgs []zuluPackage
		if err := json.Unmarshal(body, &pkgs); err != nil {
			return nil, err
		}
		for _, pkg := range pkgs {
			if !pkg.Latest || pkg.ReleaseStatus != "ga" {
				continue
			}
			v := zuluVersionString(pkg)
			out = append(out, v)
			if limit > 0 && len(out) >= limit {
				return out, nil
			}
		}
		if len(pkgs) < 50 {
			break
		}
	}
	// Zulu API returns newest first already, but deduplicate.
	seen := make(map[string]bool)
	var deduped []string
	for _, v := range out {
		if !seen[v] {
			seen[v] = true
			deduped = append(deduped, v)
		}
	}
	return deduped, nil
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
	q.Set("java_version", version)
	q.Set("page", "1")
	q.Set("page_size", "5")
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
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return nil, err
	}
	var pkgs []zuluPackage
	if err := json.Unmarshal(body, &pkgs); err != nil {
		return nil, err
	}
	// Find the best (latest GA) match.
	var best *zuluPackage
	for i := range pkgs {
		p := &pkgs[i]
		if p.ReleaseStatus != "ga" {
			continue
		}
		if best == nil || p.Latest {
			best = p
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

// zuluArchiveType returns the archive format for the current platform.
func zuluArchiveType() string {
	if runtime.GOOS == "windows" {
		return "zip"
	}
	return "tar.gz"
}
