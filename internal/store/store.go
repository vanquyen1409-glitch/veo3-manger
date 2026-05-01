// Package store owns persistence for the app: settings.json, videos.json,
// and selectors.json under %APPDATA%/veo3-manager/. All state is held in
// memory and re-written atomically; no SQLite. Mutex-safe.
//
// Responsibilities are split across:
//   - settings.go  — load/save settings
//   - videos.go    — load/save + CRUD over the videos map
//   - selectors.go — selectors.json default-write + path getter
package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"veo3-manager/internal/types"
)

// Store is the single source of truth for persisted app state. Construct via
// New(); the zero value is not usable.
type Store struct {
	mu          sync.RWMutex
	dir         string
	settings    types.Settings
	videos      map[string]types.Video
	settingFile string
	videosFile  string
	selectorCfg string
}

// New creates the on-disk config dir if missing, loads settings + videos +
// selectors, and marks any phantom "generating" rows from a crashed previous
// run as failed.
func New() (*Store, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("user config dir: %w", err)
	}
	dir := filepath.Join(cfgDir, "veo3-manager")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dir, err)
	}

	s := &Store{
		dir:         dir,
		videos:      make(map[string]types.Video),
		settingFile: filepath.Join(dir, "settings.json"),
		videosFile:  filepath.Join(dir, "videos.json"),
		selectorCfg: filepath.Join(dir, "selectors.json"),
	}

	if err := s.loadSettings(); err != nil {
		return nil, err
	}
	if err := s.loadVideos(); err != nil {
		return nil, err
	}
	if err := s.ensureSelectorConfig(); err != nil {
		return nil, err
	}
	s.markStaleGeneratingFailed()
	return s, nil
}

// Dir returns the absolute path of the on-disk config directory
// (%APPDATA%/veo3-manager/ on Windows).
func (s *Store) Dir() string { return s.dir }

// markStaleGeneratingFailed marks any video left in "generating" status (e.g.
// from a crashed previous run) as failed so it doesn't sit forever as a
// phantom in-progress row in the UI.
func (s *Store) markStaleGeneratingFailed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	now := time.Now()
	for id, v := range s.videos {
		if v.Status != types.StatusGenerating {
			continue
		}
		v.Status = types.StatusFailed
		if v.ErrorMessage == "" {
			v.ErrorMessage = "Ứng dụng kết thúc trước khi tạo xong."
		}
		v.CompletedAt = &now
		s.videos[id] = v
		changed = true
	}
	if changed {
		_ = s.saveVideosLocked()
	}
}
