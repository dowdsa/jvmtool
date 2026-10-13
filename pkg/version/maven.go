package version

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"jm/pkg/config"
)

const mavenMetadataURL = "https://repo.maven.apache.org/maven2/org/apache/maven/apache-maven/maven-metadata.xml"

// mavenMetadataTTL bounds how long a cached maven-metadata.xml is served
// without revalidation.
const mavenMetadataTTL = 5 * time.Minute

// metadataCache is a small TTL cache with If-Modified-Since revalidation for
// the maven metadata document.
type metadataCache struct {
	mu           sync.Mutex
	ttl          time.Duration
	fetchedAt    time.Time
	meta         *metadata
	lastModified string
}

// MavenSource implements Source backed by Apache Maven central.
type MavenSource struct {
	// Client, when non-nil, is used directly (tests inject fakes). Otherwise
	// the proxy-aware config.HTTPClient() is resolved per request.
	Client *http.Client
	cache  *metadataCache
}

var (
	mavenSourceOnce  sync.Once
	sharedMavenCache *metadataCache
)

// NewMavenSource returns a MavenSource sharing a process-wide metadata cache,
// so long-lived processes (e.g. the desktop app) do not re-fetch
// maven-metadata.xml on every command.
func NewMavenSource() *MavenSource {
	mavenSourceOnce.Do(func() {
		sharedMavenCache = &metadataCache{ttl: mavenMetadataTTL}
	})
	return &MavenSource{cache: sharedMavenCache}
}

func (s *MavenSource) client() *http.Client {
	if s.Client != nil {
		return s.Client
	}
	return config.HTTPClient()
}

type metadata struct {
	Versioning struct {
		Latest   string   `xml:"latest"`
		Release  string   `xml:"release"`
		Versions []string `xml:"versions>version"`
	} `xml:"versioning"`
}

func (s *MavenSource) fetchMetadata(ctx context.Context) (*metadata, error) {
	if s.cache == nil {
		return s.fetchMetadataLive(ctx, "")
	}
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	if s.cache.meta != nil && time.Since(s.cache.fetchedAt) < s.cache.ttl {
		return s.cache.meta, nil
	}
	m, err := s.fetchMetadataLive(ctx, s.cache.lastModified)
	switch {
	case errors.Is(err, errNotModified):
		s.cache.fetchedAt = time.Now()
		return s.cache.meta, nil
	case err != nil:
		// Serve stale data if the refresh fails and we have something cached.
		if s.cache.meta != nil {
			return s.cache.meta, nil
		}
		return nil, err
	}
	s.cache.meta = m
	s.cache.fetchedAt = time.Now()
	return m, nil
}

// errNotModified signals an HTTP 304 against an If-Modified-Since request.
var errNotModified = errors.New("maven metadata not modified")

// fetchMetadataLive performs the actual HTTP GET. If ifModifiedSince is
// non-empty it is sent as If-Modified-Since; a 304 yields errNotModified.
func (s *MavenSource) fetchMetadataLive(ctx context.Context, ifModifiedSince string) (*metadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mavenMetadataURL, nil)
	if err != nil {
		return nil, err
	}
	if ifModifiedSince != "" {
		req.Header.Set("If-Modified-Since", ifModifiedSince)
	}
	resp, err := s.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		return nil, errNotModified
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("maven metadata returned %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10 MB limit
	if err != nil {
		return nil, err
	}
	var m metadata
	if err := xml.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	if s.cache != nil {
		s.cache.lastModified = resp.Header.Get("Last-Modified")
	}
	return &m, nil
}

func (s *MavenSource) List(ctx context.Context, query string, limit int) ([]string, error) {
	m, err := s.fetchMetadata(ctx)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, v := range m.Versioning.Versions {
		if IsPreRelease(v) {
			continue
		}
		if query != "" && v != query && !strings.HasPrefix(v, query+".") && !strings.HasPrefix(v, query+"-") {
			continue
		}
		out = append(out, v)
	}
	// The metadata lists versions in ascending order; the Source contract is
	// "newest first", so sort descending before capping.
	sortVersionsDesc(out)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *MavenSource) Resolve(ctx context.Context, version string) (*Artifact, error) {
	versions, err := s.List(ctx, version, 1)
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("no Maven version matches %q", version)
	}
	exact := versions[0]
	filename := fmt.Sprintf("apache-maven-%s-bin.tar.gz", exact)
	base := "https://repo.maven.apache.org/maven2/org/apache/maven/apache-maven/" + exact
	sha512, err := s.fetchSHA512(ctx, base+"/"+filename+".sha512")
	if err != nil {
		return nil, fmt.Errorf("获取 Maven 校验和失败: %w", err)
	}
	return &Artifact{
		Version:  exact,
		Filename: filename,
		URL:      base + "/" + filename,
		Mirrors: []string{
			fmt.Sprintf("https://mirrors.huaweicloud.com/apache/maven/maven-3/%s/binaries/%s", exact, filename),
		},
		SHA512: sha512,
	}, nil
}

func (s *MavenSource) fetchSHA512(ctx context.Context, u string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	resp, err := s.client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checksum endpoint returned %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(body))
	if len(fields) == 0 || len(fields[0]) != 128 {
		return "", fmt.Errorf("invalid SHA512 checksum response")
	}
	return fields[0], nil
}
