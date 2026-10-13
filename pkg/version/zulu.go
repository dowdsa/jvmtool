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
	Name        string `json:"name"`
	JavaVersion []int  `json:"java_version"`
	DownloadURL string `json:"download_url"`
	PackageUUID string `json:"package_uuid"`
	Size        int64  `json:"size"`
	SHA256Hash  string `json:"sha256_hash"`
	Latest      bool   `json:"latest"`
	// ReleaseStatus is only present on the package detail endpoint; the list
	// endpoint has served slim records (without it) since Azul's 2026 change.
	ReleaseStatus string `json:"release_status"`
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
		// The API filters by plain major version (java_version=17). The old
		// bracket format ([17]) is now silently ignored and returns everything.
		// For JDK 8, query is already normalized to e.g. "8", "8u431" etc.
		majorVer := extractMajorVersion(NormalizeJDKVersion(query))
		if majorVer != "" {
			q.Set("java_version", majorVer)
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
		// release_status=ga is enforced server-side via the query param; the
		// slim list records no longer carry the field, so don't filter on it.
		if !pkg.Latest {
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
	// Extract major version for API filter (plain format, see List)
	majorVer := extractMajorVersion(version)
	if majorVer != "" {
		q.Set("java_version", majorVer)
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
	// Pick the best match: prefer a Latest non-crac build (crac variants are
	// CRIU-specialized), then Latest, then any record.
	isCrac := func(n string) bool { return strings.Contains(strings.ToLower(n), "-crac-") }
	var best *zuluPackage
	for _, pass := range []struct{ requireLatest, allowCrac bool }{
		{true, false}, {true, true}, {false, false}, {false, true},
	} {
		for i := range pkgs {
			p := &pkgs[i]
			if p.Latest != pass.requireLatest && pass.requireLatest {
				continue
			}
			if !pass.allowCrac && isCrac(p.Name) {
				continue
			}
			best = p
			break
		}
		if best != nil {
			break
		}
	}
	if best == nil {
		return nil, fmt.Errorf("no Zulu JDK matches %q for %s/%s", version, s.OS, s.Arch)
	}
	// The list endpoint no longer includes checksums; fetch the full record.
	detail, err := s.fetchPackageDetail(ctx, best.PackageUUID)
	if err != nil {
		return nil, err
	}
	if detail.SHA256Hash == "" {
		return nil, fmt.Errorf("zulu package %s has no SHA256 checksum", detail.Name)
	}
	if detail.ReleaseStatus != "" && detail.ReleaseStatus != "ga" {
		return nil, fmt.Errorf("zulu package %s is not a GA release (%s)", detail.Name, detail.ReleaseStatus)
	}
	v := zuluVersionString(*detail)
	return &Artifact{
		Version:  v,
		Filename: detail.Name,
		URL:      detail.DownloadURL,
		Size:     detail.Size,
		SHA256:   detail.SHA256Hash,
	}, nil
}

// fetchPackageDetail retrieves the full record for one package (the list
// endpoint only returns slim records without sha256_hash/release_status).
func (s *ZuluSource) fetchPackageDetail(ctx context.Context, uuid string) (*zuluPackage, error) {
	if uuid == "" {
		return nil, fmt.Errorf("zulu package missing package_uuid")
	}
	// uuid comes from the API response; keep it strictly path-safe.
	if len(uuid) > 64 || strings.ContainsAny(uuid, "/\\?# \t\n") {
		return nil, fmt.Errorf("zulu package has invalid uuid %q", uuid)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, zuluAPI+"/"+uuid, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("zulu package detail returned %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}
	var d zuluPackage
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	return &d, nil
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
