package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"veo3-manager/internal/types"
)

// makeAppWithRoot builds an *App whose store has its OutputDir set to root.
// Uses real store with temp config dir, then overrides settings.
func makeAppWithRoot(t *testing.T, root string) *App {
	t.Helper()
	tmpCfg := t.TempDir()
	t.Setenv("APPDATA", tmpCfg)
	t.Setenv("XDG_CONFIG_HOME", tmpCfg)
	app, err := NewApp()
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	if _, err := app.store.SaveSettings(types.Settings{
		ChromePath: "",
		CDPPort:    9222,
		OutputDir:  root,
	}); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	return app
}

func TestLocalFileHandler_ServesFileInRoot(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "demo.mp4")
	if err := os.WriteFile(target, []byte("hello-mp4"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := makeAppWithRoot(t, root)
	h := localFileHandler(app)

	req := httptest.NewRequest(http.MethodGet, "/localfile/"+filepath.ToSlash(target), nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (body=%q)", rr.Code, rr.Body.String())
	}
	if got := rr.Body.String(); got != "hello-mp4" {
		t.Errorf("body mismatch: %q", got)
	}
}

func TestLocalFileHandler_RejectsTraversal(t *testing.T) {
	root := t.TempDir()
	outsideDir := t.TempDir()
	outside := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outside, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := makeAppWithRoot(t, root)
	h := localFileHandler(app)

	req := httptest.NewRequest(http.MethodGet, "/localfile/"+filepath.ToSlash(outside), nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("traversal should be 403, got %d body=%q", rr.Code, rr.Body.String())
	}
}

func TestLocalFileHandler_RejectsDotDotInPath(t *testing.T) {
	root := t.TempDir()
	app := makeAppWithRoot(t, root)
	h := localFileHandler(app)

	// Try to escape with ..
	target := filepath.Join(root, "..", "evil.txt")
	req := httptest.NewRequest(http.MethodGet, "/localfile/"+filepath.ToSlash(target), nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code == http.StatusOK {
		t.Fatalf("dot-dot escape should not return 200, got %d", rr.Code)
	}
}

func TestLocalFileHandler_NotFoundOutsidePrefix(t *testing.T) {
	root := t.TempDir()
	app := makeAppWithRoot(t, root)
	h := localFileHandler(app)

	req := httptest.NewRequest(http.MethodGet, "/something/else.mp4", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("non-prefix paths should be 404, got %d", rr.Code)
	}
}

func TestLocalFileHandler_PercentEncodedPathWorks(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "video with spaces.mp4")
	if err := os.WriteFile(target, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	app := makeAppWithRoot(t, root)
	h := localFileHandler(app)

	encoded := strings.ReplaceAll(filepath.ToSlash(target), " ", "%20")
	req := httptest.NewRequest(http.MethodGet, "/localfile/"+encoded, nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("encoded path: want 200, got %d body=%q", rr.Code, rr.Body.String())
	}
}
