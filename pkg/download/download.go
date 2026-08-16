package download

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"jm/pkg/config"
)

// ErrCancelled is returned when a download is cancelled via context.
var ErrCancelled = errors.New("download cancelled")

// ProgressCallback receives downloaded/total bytes and rate (bytes/sec).
type ProgressCallback func(done, total, rate int64)

// Downloader fetches a URL to a local file with resume support.
type Downloader struct {
	Client *http.Client
}

func New() *Downloader {
	return &Downloader{Client: config.HTTPClient()}
}

// Download writes url to path. If path exists, it resumes from its size.
// ctx allows cancellation. cb is optional and called with (done, total, rate).
func (d *Downloader) Download(ctx context.Context, url, path string, cb ProgressCallback) error {
	done := int64(0)
	if fi, err := os.Stat(path); err == nil {
		done = fi.Size()
	}

	start := time.Now()
	windowStart := time.Now()
	windowBytes := int64(0)
	rate := int64(0)

	for attempt := 0; attempt < 3; attempt++ {
		if ctx.Err() != nil {
			return ErrCancelled
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		if done > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", done))
		}
		resp, err := d.Client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return ErrCancelled
			}
			return err
		}
		switch {
		case resp.StatusCode == http.StatusOK:
			done = 0 // server ignored Range, restart file
		case resp.StatusCode == http.StatusPartialContent:
		case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable:
			resp.Body.Close()
			if cb != nil {
				cb(done, done, 0)
			}
			return nil // already fully downloaded
		default:
			resp.Body.Close()
			return fmt.Errorf("http %s", resp.Status)
		}

		total := done
		if resp.ContentLength > 0 {
			total += resp.ContentLength
		}

		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			resp.Body.Close()
			return err
		}
		if done > 0 {
			if _, err := f.Seek(done, io.SeekStart); err != nil {
				f.Close()
				resp.Body.Close()
				return err
			}
		}

		buf := make([]byte, 256*1024)
		for {
			n, rerr := resp.Body.Read(buf)
			if n > 0 {
				if _, werr := f.Write(buf[:n]); werr != nil {
					f.Close()
					resp.Body.Close()
					return werr
				}
				done += int64(n)
				windowBytes += int64(n)
				if cb != nil {
					now := time.Now()
					elapsed := now.Sub(windowStart)
					if elapsed >= time.Second {
						rate = int64(float64(windowBytes) / elapsed.Seconds())
						windowBytes = 0
						windowStart = now
					}
					cb(done, total, rate)
				}
			}
			if rerr == io.EOF {
				break
			}
			if rerr != nil {
				f.Close()
				resp.Body.Close()
				if ctx.Err() != nil {
					return ErrCancelled
				}
				return rerr
			}
			if ctx.Err() != nil {
				f.Close()
				resp.Body.Close()
				return ErrCancelled
			}
		}
		f.Close()
		resp.Body.Close()

		if ctx.Err() != nil {
			return ErrCancelled
		}
		if resp.ContentLength > 0 && done != total {
			continue // retry
		}
		_ = start
		return nil
	}
	return fmt.Errorf("failed to fully download %s", url)
}

// VerifySHA256 checks the file against an expected hex sha256 digest.
func VerifySHA256(path, expected string) (bool, error) {
	if expected == "" {
		return true, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	got := hex.EncodeToString(h.Sum(nil))
	return strings.EqualFold(got, expected), nil
}

// VerifySHA512 checks the file against an expected hex sha512 digest.
func VerifySHA512(path, expected string) (bool, error) {
	if expected == "" {
		return true, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	h := sha512.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	got := hex.EncodeToString(h.Sum(nil))
	return strings.EqualFold(got, expected), nil
}

// ProgressBar renders a simple line-based progress bar.
type ProgressBar struct {
	start  time.Time
	last   time.Time
	label  string
	isTTY  bool
	width  int
}

// NewProgressBar creates a progress bar. label shown before the bar.
func NewProgressBar(label string) *ProgressBar {
	return &ProgressBar{
		label: label,
		start: time.Now(),
		last:  time.Now(),
		isTTY: isTerminal(os.Stdout),
		width: 40,
	}
}

func (p *ProgressBar) Callback() ProgressCallback {
	return func(done, total, _ int64) {
		now := time.Now()
		if now.Sub(p.last) < 100*time.Millisecond {
			return
		}
		p.last = now
		if p.isTTY {
			p.renderTTY(done, total)
		} else {
			if total > 0 && done% (total/100+1) == 0 {
				fmt.Printf("\r%s %d%%", p.label, done*100/total)
			}
		}
	}
}

func (p *ProgressBar) renderTTY(done, total int64) {
	pct := float64(0)
	if total > 0 {
		pct = float64(done) / float64(total)
	}
	filled := int(pct * float64(p.width))
	bar := strings.Repeat("=", filled) + strings.Repeat("-", p.width-filled)
	elapsed := time.Since(p.start)
	rate := float64(0)
	if elapsed.Seconds() > 0 {
		rate = float64(done) / elapsed.Seconds()
	}
	eta := "--"
	if rate > 0 && total > 0 {
		eta = fmt.Sprintf("%d s", int(float64(total-done)/rate))
	}
	fmt.Fprintf(os.Stdout, "\r%s [%s] %5.1f%% %s/%s %s/s ETA %s",
		p.label, bar, pct*100,
		humanSize(done), humanSize(total), humanSize(int64(rate)), eta)
}

// Done clears the progress line.
func (p *ProgressBar) Done() {
	if p.isTTY {
		fmt.Fprint(os.Stdout, "\r"+strings.Repeat(" ", 100)+"\r")
	} else {
		fmt.Println()
	}
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
