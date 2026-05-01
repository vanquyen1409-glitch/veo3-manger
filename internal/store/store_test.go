package store

import (
	"path/filepath"
	"testing"
	"time"

	"veo3-manager/internal/types"
)

// newTestStore builds a fresh Store rooted at a temp directory by overriding
// XDG_CONFIG_HOME (Linux) / APPDATA (Windows) for the duration of the test.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	s, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func TestStore_DefaultsOnFirstRun(t *testing.T) {
	s := newTestStore(t)
	got := s.GetSettings()
	if got.CDPPort != 9222 {
		t.Errorf("CDPPort default: want 9222, got %d", got.CDPPort)
	}
	if got.SelectorConfigPath == "" {
		t.Error("SelectorConfigPath should be set on first run")
	}
	if filepath.Base(got.SelectorConfigPath) != "selectors.json" {
		t.Errorf("selector path should end in selectors.json, got %s", got.SelectorConfigPath)
	}
	if got.OutputDir == "" {
		t.Error("OutputDir should be set to a default on first run")
	}
}

func TestStore_SaveSettingsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)

	s, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	updated, err := s.SaveSettings(types.Settings{
		ChromePath: `C:\Browser\chrome.exe`,
		CDPPort:    9333,
		OutputDir:  `D:\Videos`,
	})
	if err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	if updated.CDPPort != 9333 {
		t.Errorf("port not preserved: %d", updated.CDPPort)
	}
	if updated.SelectorConfigPath == "" {
		t.Error("SelectorConfigPath must be re-injected after save")
	}

	// Reopen, ensure persisted
	s2, err := New()
	if err != nil {
		t.Fatalf("New 2: %v", err)
	}
	got := s2.GetSettings()
	if got.ChromePath != `C:\Browser\chrome.exe` {
		t.Errorf("ChromePath not persisted: %q", got.ChromePath)
	}
	if got.CDPPort != 9333 {
		t.Errorf("CDPPort not persisted: %d", got.CDPPort)
	}
	if got.OutputDir != `D:\Videos` {
		t.Errorf("OutputDir not persisted: %q", got.OutputDir)
	}
}

func TestStore_SaveSettingsZeroPortFallsBackTo9222(t *testing.T) {
	s := newTestStore(t)
	out, err := s.SaveSettings(types.Settings{CDPPort: 0, OutputDir: "x"})
	if err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	if out.CDPPort != 9222 {
		t.Errorf("zero port should fall back to 9222, got %d", out.CDPPort)
	}
}

func TestStore_VideosCRUD(t *testing.T) {
	s := newTestStore(t)
	v := types.Video{
		ID:          "v_test1",
		Prompt:      "a cat in a forest",
		AspectRatio: types.AspectLandscape,
		Resolution:  types.Res720p,
		Duration:    6,
		Tags:        []string{"cinematic"},
		Status:      types.StatusGenerating,
		CreatedAt:   time.Now(),
	}
	if err := s.UpsertVideo(v); err != nil {
		t.Fatalf("UpsertVideo: %v", err)
	}
	list := s.ListVideos()
	if len(list) != 1 || list[0].ID != "v_test1" {
		t.Fatalf("ListVideos: want 1 entry, got %v", list)
	}

	updated, err := s.UpdateStatus("v_test1", types.StatusCompleted, "")
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if updated.Status != types.StatusCompleted {
		t.Errorf("status not updated, got %v", updated.Status)
	}
	if updated.CompletedAt == nil {
		t.Error("CompletedAt should be set on terminal status")
	}

	if err := s.DeleteVideo("v_test1"); err != nil {
		t.Fatalf("DeleteVideo: %v", err)
	}
	if got, ok := s.GetVideo("v_test1"); ok {
		t.Errorf("video should be gone after delete, got %v", got)
	}

	if err := s.DeleteVideo("nope"); err == nil {
		t.Error("deleting missing id should error")
	}
}

func TestStore_StaleGeneratingMarkedFailedOnLoad(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)

	s1, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := s1.UpsertVideo(types.Video{
		ID:        "stale",
		Prompt:    "unfinished",
		Status:    types.StatusGenerating,
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("UpsertVideo: %v", err)
	}

	// Reopen — markStaleGeneratingFailed should run.
	s2, err := New()
	if err != nil {
		t.Fatalf("New 2: %v", err)
	}
	got, ok := s2.GetVideo("stale")
	if !ok {
		t.Fatal("video disappeared")
	}
	if got.Status != types.StatusFailed {
		t.Errorf("stale generating should be marked failed, got %v", got.Status)
	}
	if got.ErrorMessage == "" {
		t.Error("failed sweep should set an error message")
	}
	if got.CompletedAt == nil {
		t.Error("CompletedAt should be set when marking stale as failed")
	}
}

func TestStore_ListVideosSortedNewestFirst(t *testing.T) {
	s := newTestStore(t)
	t1 := time.Now().Add(-2 * time.Hour)
	t2 := time.Now().Add(-1 * time.Hour)
	t3 := time.Now()
	if err := s.UpsertVideo(types.Video{ID: "a", Status: types.StatusCompleted, CreatedAt: t1}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertVideo(types.Video{ID: "b", Status: types.StatusCompleted, CreatedAt: t3}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertVideo(types.Video{ID: "c", Status: types.StatusCompleted, CreatedAt: t2}); err != nil {
		t.Fatal(err)
	}
	list := s.ListVideos()
	if len(list) != 3 {
		t.Fatalf("want 3, got %d", len(list))
	}
	if list[0].ID != "b" || list[1].ID != "c" || list[2].ID != "a" {
		t.Errorf("sort order wrong: got %s, %s, %s", list[0].ID, list[1].ID, list[2].ID)
	}
}
