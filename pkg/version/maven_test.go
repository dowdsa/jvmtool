package version

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMavenResolveUsesNewestMatchingStableVersion(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := ""
		if strings.Contains(r.URL.Path, "maven-metadata.xml") {
			body = `<metadata><versioning><release>3.9.11</release><versions><version>3.9.0</version><version>3.9.11</version><version>4.0.0-beta-1</version></versions></versioning></metadata>`
		} else {
			body = strings.Repeat("a", 128) + "  apache-maven-3.9.11-bin.tar.gz"
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}

	source := &MavenSource{Client: client}
	artifact, err := source.Resolve(context.Background(), "3.9")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if artifact.Version != "3.9.11" {
		t.Fatalf("Resolve selected %q, want 3.9.11", artifact.Version)
	}
	if artifact.SHA512 != strings.Repeat("a", 128) {
		t.Fatalf("Resolve returned malformed checksum %q", artifact.SHA512)
	}
}

func TestMavenResolveRejectsUnknownVersion(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `<metadata><versioning><release>3.9.11</release><versions><version>3.9.11</version></versions></versioning></metadata>`
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}

	_, err := (&MavenSource{Client: client}).Resolve(context.Background(), "does-not-exist")
	if err == nil {
		t.Fatal("Resolve should reject an unknown version instead of installing latest")
	}
}

func TestMavenMetadataCacheServesWithinTTL(t *testing.T) {
	var hits int
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "maven-metadata.xml") {
			hits++
		}
		body := `<metadata><versioning><release>3.9.11</release><versions><version>3.9.11</version></versions></versioning></metadata>`
		if strings.Contains(r.URL.Path, "maven-metadata.xml") {
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(strings.Repeat("a", 128))), Header: make(http.Header)}, nil
	})}
	source := &MavenSource{Client: client, cache: &metadataCache{ttl: time.Hour}}

	for i := 0; i < 3; i++ {
		if _, err := source.List(context.Background(), "", 0); err != nil {
			t.Fatalf("List %d: %v", i, err)
		}
	}
	if hits != 1 {
		t.Fatalf("metadata fetched %d times, want 1 (cache miss within TTL)", hits)
	}
}

func TestMavenMetadataCacheRevalidatesWithIfModifiedSince(t *testing.T) {
	var lastModifiedSent, fullFetches int
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if !strings.Contains(r.URL.Path, "maven-metadata.xml") {
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(strings.Repeat("a", 128))), Header: make(http.Header)}, nil
		}
		if v := r.Header.Get("If-Modified-Since"); v != "" {
			lastModifiedSent++
			return &http.Response{StatusCode: http.StatusNotModified, Status: "304 Not Modified", Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
		fullFetches++
		h := make(http.Header)
		h.Set("Last-Modified", "Wed, 01 Jan 2025 00:00:00 GMT")
		body := `<metadata><versioning><release>3.9.11</release><versions><version>3.9.11</version></versions></versioning></metadata>`
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body)), Header: h}, nil
	})}
	cache := &metadataCache{ttl: time.Nanosecond}
	source := &MavenSource{Client: client, cache: cache}

	for i := 0; i < 3; i++ {
		if _, err := source.List(context.Background(), "", 0); err != nil {
			t.Fatalf("List %d: %v", i, err)
		}
	}
	if fullFetches != 1 {
		t.Fatalf("full fetches = %d, want 1", fullFetches)
	}
	if lastModifiedSent != 2 {
		t.Fatalf("conditional fetches = %d, want 2", lastModifiedSent)
	}
}

func TestMavenMetadataCacheServesStaleOnFetchError(t *testing.T) {
	fail := false
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if fail && strings.Contains(r.URL.Path, "maven-metadata.xml") {
			return nil, errors.New("network down")
		}
		body := `<metadata><versioning><release>3.9.11</release><versions><version>3.9.11</version></versions></versioning></metadata>`
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	cache := &metadataCache{ttl: time.Nanosecond}
	source := &MavenSource{Client: client, cache: cache}

	if _, err := source.List(context.Background(), "", 0); err != nil {
		t.Fatalf("first List: %v", err)
	}
	fail = true
	versions, err := source.List(context.Background(), "", 0)
	if err != nil {
		t.Fatalf("List should serve stale cache on error, got %v", err)
	}
	if len(versions) != 1 || versions[0] != "3.9.11" {
		t.Fatalf("stale cache returned %v", versions)
	}
}
