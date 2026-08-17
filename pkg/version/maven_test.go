package version

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
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
