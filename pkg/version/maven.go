package version

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"jm/pkg/config"
)

const mavenMetadataURL = "https://repo.maven.apache.org/maven2/org/apache/maven/apache-maven/maven-metadata.xml"

// MavenSource implements Source backed by Apache Maven central.
type MavenSource struct {
	Client *http.Client
}

func NewMavenSource() *MavenSource {
	return &MavenSource{Client: config.HTTPClient()}
}

type metadata struct {
	Versioning struct {
		Latest   string   `xml:"latest"`
		Release  string   `xml:"release"`
		Versions []string `xml:"versions>version"`
	} `xml:"versioning"`
}

func (s *MavenSource) fetchMetadata(ctx context.Context) (*metadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mavenMetadataURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("maven metadata returned %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var m metadata
	if err := xml.Unmarshal(body, &m); err != nil {
		return nil, err
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
		if query != "" && !strings.Contains(v, query) {
			continue
		}
		out = append(out, v)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

var mavenVersionRe = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

func (s *MavenSource) Resolve(ctx context.Context, version string) (*Artifact, error) {
	m, err := s.fetchMetadata(ctx)
	if err != nil {
		return nil, err
	}
	var exact string
	for _, v := range m.Versioning.Versions {
		if v == version || strings.HasPrefix(v, version+".") || strings.HasPrefix(v, version+"-") {
			exact = v
			break
		}
	}
	if exact == "" && m.Versioning.Release != "" {
		exact = m.Versioning.Release
	}
	if exact == "" {
		return nil, fmt.Errorf("no Maven version matches %q", version)
	}
	filename := fmt.Sprintf("apache-maven-%s-bin.tar.gz", exact)
	base := "https://repo.maven.apache.org/maven2/org/apache/maven/apache-maven/" + exact
	return &Artifact{
		Version:  exact,
		Filename: filename,
		URL:      base + "/" + filename,
		Mirrors: []string{
			fmt.Sprintf("https://mirrors.huaweicloud.com/apache/maven/maven-3/%s/binaries/%s", exact, filename),
		},
		SHA512: s.fetchSHA512(ctx, base+"/"+filename+".sha512"),
	}, nil
}

func (s *MavenSource) fetchSHA512(ctx context.Context, u string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return ""
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(body))
}
