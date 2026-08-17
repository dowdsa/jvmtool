package version

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
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
	resp, err := s.Client.Do(req)
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
