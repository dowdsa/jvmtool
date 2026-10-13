package download

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
)

func makeData(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('a' + i%26)
	}
	return b
}

// rangeServer serves data with correct Range handling. If abortBefore>0, the
// first attempt delivers only abortBefore bytes and then aborts the
// connection mid-stream.
func rangeServer(data []byte, abortBefore int64, abortEvery bool) (*httptest.Server, func() int) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		start := int64(0)
		status := http.StatusOK
		if rng := r.Header.Get("Range"); rng != "" {
			lo := strings.TrimPrefix(rng, "bytes=")
			s, _, _ := strings.Cut(lo, "-")
			v, err := strconv.ParseInt(s, 10, 64)
			if err != nil || v < 0 || v > int64(len(data)) {
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return
			}
			start = v
			status = http.StatusPartialContent
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(data)-1, len(data)))
		}
		rest := data[start:]
		w.Header().Set("Content-Length", strconv.Itoa(len(rest)))
		w.WriteHeader(status)
		abort := abortBefore > 0 && (abortEvery || attempts == 1)
		limit := int64(len(rest))
		if abort && abortBefore < limit {
			limit = abortBefore
		}
		w.Write(rest[:limit])
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		if abort && limit < int64(len(rest)) {
			panic(http.ErrAbortHandler)
		}
	}))
	return srv, func() int { return attempts }
}

func TestDownloadFullFile(t *testing.T) {
	data := makeData(2048)
	srv, _ := rangeServer(data, 0, false)
	defer srv.Close()

	path := t.TempDir() + "/file.bin"
	d := &Downloader{Client: srv.Client()}
	if err := d.Download(context.Background(), srv.URL, path, nil); err != nil {
		t.Fatalf("Download: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(data) {
		t.Fatalf("downloaded content mismatch: got %d bytes, want %d", len(got), len(data))
	}
}

func TestDownloadResumesFromPartialFile(t *testing.T) {
	data := makeData(4096)
	srv, attempts := rangeServer(data, 0, false)
	defer srv.Close()

	path := t.TempDir() + "/file.bin"
	if err := os.WriteFile(path, data[:1000], 0o600); err != nil {
		t.Fatal(err)
	}
	d := &Downloader{Client: srv.Client()}
	if err := d.Download(context.Background(), srv.URL, path, nil); err != nil {
		t.Fatalf("Download: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(data) {
		t.Fatal("resumed file does not match full content")
	}
	if attempts() != 1 {
		t.Fatalf("attempts = %d, want 1", attempts())
	}
}

func TestDownloadRetriesMidStreamAbortWithRange(t *testing.T) {
	data := makeData(4096)
	srv, attempts := rangeServer(data, 500, false) // first attempt aborts after 500 bytes
	defer srv.Close()

	path := t.TempDir() + "/file.bin"
	d := &Downloader{Client: srv.Client()}
	if err := d.Download(context.Background(), srv.URL, path, nil); err != nil {
		t.Fatalf("Download should recover via retry: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(data) {
		t.Fatalf("content corrupted after retry: got %d bytes, want %d", len(got), len(data))
	}
	if attempts() < 2 {
		t.Fatalf("expected at least 2 attempts, got %d", attempts())
	}
}

func TestDownloadErrorAfterMaxAttempts(t *testing.T) {
	data := makeData(4096)
	srv, attempts := rangeServer(data, 100, true) // every attempt aborts early
	defer srv.Close()

	path := t.TempDir() + "/file.bin"
	d := &Downloader{Client: srv.Client()}
	err := d.Download(context.Background(), srv.URL, path, nil)
	if err == nil {
		t.Fatal("Download should fail after 3 aborted attempts")
	}
	if attempts() != 3 {
		t.Fatalf("attempts = %d, want 3", attempts())
	}
}

func TestDownloadServerIgnoresRangeTruncatesStaleTail(t *testing.T) {
	data := makeData(2048)
	// Server never honours Range: always 200 with the full body.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer srv.Close()

	path := t.TempDir() + "/file.bin"
	// Stale tail longer than the real content must not survive.
	if err := os.WriteFile(path, makeData(3000), 0o600); err != nil {
		t.Fatal(err)
	}
	d := &Downloader{Client: srv.Client()}
	if err := d.Download(context.Background(), srv.URL, path, nil); err != nil {
		t.Fatalf("Download: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(data) {
		t.Fatalf("stale tail not truncated: got %d bytes, want %d", len(got), len(data))
	}
}

func TestDownloadCompleteFileGets416AndSucceeds(t *testing.T) {
	data := makeData(1024)
	srv, attempts := rangeServer(data, 0, false)
	defer srv.Close()

	path := t.TempDir() + "/file.bin"
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	d := &Downloader{Client: srv.Client()}
	if err := d.Download(context.Background(), srv.URL, path, nil); err != nil {
		t.Fatalf("already-complete download should succeed: %v", err)
	}
	if attempts() != 1 {
		t.Fatalf("attempts = %d, want 1", attempts())
	}
}

func TestDownloadContextCancel(t *testing.T) {
	data := makeData(1 << 20)
	srv, _ := rangeServer(data, 0, false)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	path := t.TempDir() + "/file.bin"
	d := &Downloader{Client: srv.Client()}
	go cancel()
	err := d.Download(ctx, srv.URL, path, nil)
	if err != nil && err != ErrCancelled {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDownloadProgressCallback(t *testing.T) {
	data := makeData(600)
	srv, _ := rangeServer(data, 0, false)
	defer srv.Close()

	path := t.TempDir() + "/file.bin"
	var lastDone, lastTotal int64
	d := &Downloader{Client: srv.Client()}
	err := d.Download(context.Background(), srv.URL, path, func(done, total, rate int64) {
		lastDone, lastTotal = done, total
	})
	if err != nil {
		t.Fatal(err)
	}
	if lastDone != int64(len(data)) || lastTotal != int64(len(data)) {
		t.Fatalf("final progress = %d/%d, want %d/%d", lastDone, lastTotal, len(data), len(data))
	}
}
