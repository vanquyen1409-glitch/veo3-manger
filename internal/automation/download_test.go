package automation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestIsSignedVideoURL(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"https://storage.googleapis.com/foo/bar.mp4?Signature=abc", true},
		{"https://lh3.googleusercontent.com/abc123", true},
		{"https://STORAGE.GOOGLEAPIS.COM/foo.mp4", true}, // case-insensitive
		{"https://labs.google/redirect/xyz", false},
		{"https://example.com/video.mp4", false},
		{"", false},
		{"not a url", false},
	}
	for _, tc := range cases {
		got := IsSignedVideoURL(tc.url)
		if got != tc.want {
			t.Errorf("IsSignedVideoURL(%q) = %v, want %v", tc.url, got, tc.want)
		}
	}
}

func TestDownloadFile_StreamsToDisk(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("FAKE_MP4_CONTENT_HERE"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "out.mp4")
	if err := downloadFile(context.Background(), srv.URL, dest); err != nil {
		t.Fatalf("downloadFile: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "FAKE_MP4_CONTENT_HERE" {
		t.Errorf("content mismatch: %q", string(got))
	}
}

func TestDownloadFile_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("denied"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "out.mp4")
	err := downloadFile(context.Background(), srv.URL, dest)
	if err == nil {
		t.Fatal("expected HTTP error")
	}
	// .part file should have been cleaned up
	if _, err := os.Stat(dest + ".part"); !os.IsNotExist(err) {
		t.Error(".part file should be cleaned up on error")
	}
	// Final dest should not exist
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Error("dest file should not exist after error")
	}
}

func TestDownloadFile_AtomicRename(t *testing.T) {
	// Verify .part is renamed to dest, not left behind.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "video.mp4")
	if err := downloadFile(context.Background(), srv.URL, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dest + ".part"); !os.IsNotExist(err) {
		t.Error(".part should be removed after rename")
	}
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("final dest should exist: %v", err)
	}
}
