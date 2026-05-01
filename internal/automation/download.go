package automation

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// IsSignedVideoURL reports whether u looks like a Google-issued direct video
// URL — either a Cloud Storage signed URL (storage.googleapis.com with a
// Signature query param) or a Google Front End / FIFE URL on
// lh3.googleusercontent.com.
//
// Pure function so it can be unit-tested without a browser.
func IsSignedVideoURL(u string) bool {
	if u == "" {
		return false
	}
	low := strings.ToLower(u)
	if strings.Contains(low, "storage.googleapis.com") {
		return true
	}
	if strings.Contains(low, "lh3.googleusercontent.com") {
		return true
	}
	return false
}

// DownloadVideo fetches the MP4 backing a poll-response redirectURL and
// writes it to destPath. It works by:
//
//  1. Subscribing to chromedp Network response events on the existing browser.
//  2. Navigating to redirectURL — the Google Labs server issues a 302/redirect
//     chain ending at a signed Cloud Storage URL.
//  3. Capturing the first response URL whose host matches a known signed-URL
//     pattern (storage.googleapis.com or lh3.googleusercontent.com).
//  4. Performing a plain HTTP GET on that URL — signed URLs carry their own
//     auth in the query string, so cookies / Bearer headers are not required.
//  5. Streaming the body to destPath.
//
// The browser tab is left at the redirected URL; caller may close it. If no
// signed URL is observed within `timeout`, returns an error.
func (b *Browser) DownloadVideo(ctx context.Context, redirectURL, destPath string, timeout time.Duration) error {
	if b == nil || b.ctx == nil {
		return errors.New("browser chưa kết nối")
	}
	if redirectURL == "" {
		return errors.New("redirectURL rỗng")
	}
	if destPath == "" {
		return errors.New("destPath rỗng")
	}
	if timeout <= 0 {
		timeout = 90 * time.Second
	}

	// Ensure destination directory exists.
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("tạo thư mục lưu: %w", err)
	}

	// Wire up a listener that captures the first signed video URL.
	var (
		mu        sync.Mutex
		signedURL string
		found     = make(chan struct{})
	)
	listenerCtx, cancelListener := context.WithCancel(b.ctx)
	defer cancelListener()

	chromedp.ListenTarget(listenerCtx, func(ev any) {
		resp, ok := ev.(*network.EventResponseReceived)
		if !ok || resp.Response == nil {
			return
		}
		url := resp.Response.URL
		if !IsSignedVideoURL(url) {
			return
		}
		mu.Lock()
		if signedURL == "" {
			signedURL = url
			close(found)
		}
		mu.Unlock()
	})

	// Enable Network domain + navigate. We don't WaitVisible — the page may
	// be the video itself (no DOM body), so just kick off navigation.
	navCtx, cancelNav := context.WithTimeout(b.ctx, timeout)
	defer cancelNav()
	if err := chromedp.Run(navCtx,
		network.Enable(),
		chromedp.Navigate(redirectURL),
	); err != nil {
		// Navigation may fail if the server returns the file directly with
		// non-HTML content-type. Don't bail — wait to see if the listener
		// caught something useful first.
		select {
		case <-found:
		case <-time.After(2 * time.Second):
			return fmt.Errorf("điều hướng đến redirect URL: %w", err)
		}
	}

	// Wait for the listener to fire or timeout.
	select {
	case <-found:
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(timeout):
		return fmt.Errorf("không bắt được signed URL trong %v", timeout)
	}

	mu.Lock()
	captured := signedURL
	mu.Unlock()
	if captured == "" {
		return errors.New("signed URL rỗng sau capture")
	}

	return downloadFile(ctx, captured, destPath)
}

// downloadFile performs a plain HTTP GET and streams the body to destPath.
// Used by DownloadVideo once the signed URL has been captured.
func downloadFile(ctx context.Context, url, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("GET signed URL: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("signed URL trả HTTP %d", resp.StatusCode)
	}

	// Write to a `.part` sibling first, then atomic-rename to destPath on
	// success. This avoids leaving half-written .mp4 files behind on crash.
	partPath := destPath + ".part"
	f, err := os.Create(partPath)
	if err != nil {
		return fmt.Errorf("tạo file đích: %w", err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(partPath)
		return fmt.Errorf("ghi file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(partPath)
		return err
	}
	if err := os.Rename(partPath, destPath); err != nil {
		_ = os.Remove(partPath)
		return fmt.Errorf("rename file đích: %w", err)
	}
	return nil
}
