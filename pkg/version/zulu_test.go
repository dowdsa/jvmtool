package version

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// Slim list record as served by Azul since 2026 (no release_status/sha256).
const zuluListSample = `[
 {"name":"zulu17.68.203-ca-crac-jdk17.0.20.1-linux_x64.tar.gz","java_version":[17,0,20,1],"download_url":"https://cdn.azul.com/zulu/bin/crac.tar.gz","package_uuid":"11111111-1111-1111-1111-111111111111","latest":true},
 {"name":"zulu17.68.203-ca-jdk17.0.20.1-linux_x64.tar.gz","java_version":[17,0,20,1],"download_url":"https://cdn.azul.com/zulu/bin/plain.tar.gz","package_uuid":"22222222-2222-2222-2222-222222222222","latest":true},
 {"name":"zulu8.90-ca-jdk8.0.472-linux_x64.tar.gz","java_version":[8,0,472],"download_url":"https://cdn.azul.com/zulu/bin/zulu8.tar.gz","package_uuid":"33333333-3333-3333-3333-333333333333","latest":true}
]`

const zuluDetailSample = `{"name":"zulu17.68.203-ca-jdk17.0.20.1-linux_x64.tar.gz","java_version":[17,0,20,1],"download_url":"https://cdn.azul.com/zulu/bin/plain.tar.gz","package_uuid":"22222222-2222-2222-2222-222222222222","sha256_hash":"deadbeef","size":123,"release_status":"ga","latest":true}`

func zuluSourceCapturing(t *testing.T, captured *[]string) *ZuluSource {
	t.Helper()
	return &ZuluSource{
		Arch: "x64",
		OS:   "linux",
		Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			*captured = append(*captured, r.URL.String())
			body := zuluListSample
			if strings.HasPrefix(r.URL.Path, "/metadata/v1/zulu/packages/") {
				body = zuluDetailSample
			}
			return &http.Response{
				StatusCode: http.StatusOK, Status: "200 OK",
				Body:   io.NopCloser(strings.NewReader(body)),
				Header: make(http.Header),
			}, nil
		})},
	}
}

func TestZuluListSendsPlainJavaVersionFilter(t *testing.T) {
	var urls []string
	versions, err := zuluSourceCapturing(t, &urls).List(context.Background(), "17", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(urls) != 1 || !strings.Contains(urls[0], "java_version=17") {
		t.Fatalf("request %v must filter java_version=17", urls)
	}
	if strings.Contains(urls[0], "%5B") {
		t.Fatalf("bracket format is silently ignored by Azul API: %q", urls[0])
	}
	// Slim records lack release_status; the two 17.x entries dedupe to one.
	if len(versions) != 1 || versions[0] != "17.0.20.1" {
		t.Fatalf("versions = %v, want [17.0.20.1]", versions)
	}
}

func TestZuluResolveFetchesDetailForChecksum(t *testing.T) {
	var urls []string
	art, err := zuluSourceCapturing(t, &urls).Resolve(context.Background(), "17")
	if err != nil {
		t.Fatal(err)
	}
	if len(urls) != 2 {
		t.Fatalf("expected list + detail requests, got %v", urls)
	}
	// Non-crac preferred: detail must be fetched for the plain package uuid.
	if !strings.Contains(urls[1], "22222222-2222-2222-2222-222222222222") {
		t.Fatalf("second request should fetch non-crac package, got %q", urls[1])
	}
	if art.SHA256 != "deadbeef" {
		t.Fatalf("artifact SHA256 = %q, want deadbeef", art.SHA256)
	}
	if art.Version != "17.0.20.1" {
		t.Fatalf("artifact version = %q", art.Version)
	}
}

func TestZuluJDK8QueryUsesMajorVersion(t *testing.T) {
	var urls []string
	if _, err := zuluSourceCapturing(t, &urls).List(context.Background(), "8u431", 0); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(urls[0], "java_version=8") {
		t.Fatalf("8u431 must filter java_version=8, got %q", urls[0])
	}
}

func TestZuluResolveRejectsMissingUUID(t *testing.T) {
	var urls []string
	src := zuluSourceCapturing(t, &urls)
	srcList := `[{"name":"x.tar.gz","java_version":[17,0,1],"download_url":"https://cdn.azul.com/x.tar.gz","latest":true}]`
	src.Client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := srcList
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	if _, err := src.Resolve(context.Background(), "17"); err == nil {
		t.Fatal("resolve must fail when package_uuid is missing (no checksum source)")
	}
}
