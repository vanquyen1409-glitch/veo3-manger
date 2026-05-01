package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"veo3-manager/internal/types"
)

// defaultOutputDir returns the platform-appropriate fallback for video
// output: %USERPROFILE%/Videos/Veo3Manager on Windows, or a "videos"
// subfolder under the config dir if USERPROFILE is missing.
func (s *Store) defaultOutputDir() string {
	home := os.Getenv("USERPROFILE")
	if home == "" {
		return filepath.Join(s.dir, "videos")
	}
	return filepath.Join(home, "Videos", "Veo3Manager")
}

// loadSettings reads settings.json from disk, falling back to defaults if
// the file doesn't exist yet (and writing the defaults so subsequent runs
// see a real file). Always normalises CDPPort and SelectorConfigPath.
func (s *Store) loadSettings() error {
	defaultOut := s.defaultOutputDir()
	s.settings = types.Settings{
		ChromePath:         "",
		CDPPort:            9222,
		SelectorConfigPath: s.selectorCfg,
		OutputDir:          defaultOut,
	}

	data, err := os.ReadFile(s.settingFile)
	if err != nil {
		if os.IsNotExist(err) {
			return s.saveSettingsLocked()
		}
		return fmt.Errorf("read settings: %w", err)
	}
	var loaded types.Settings
	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("parse settings: %w", err)
	}
	if loaded.CDPPort == 0 {
		loaded.CDPPort = 9222
	}
	loaded.SelectorConfigPath = s.selectorCfg
	if loaded.OutputDir == "" {
		loaded.OutputDir = s.settings.OutputDir
	}
	s.settings = loaded
	return nil
}

// saveSettingsLocked writes the current in-memory settings to disk. Caller
// must hold s.mu (write lock) — the *Locked suffix mirrors that contract.
// Uses atomic write so a crash mid-write can't corrupt settings.json.
func (s *Store) saveSettingsLocked() error {
	return writeJSONAtomic(s.settingFile, s.settings)
}

// GetSettings returns a copy of the current settings (RLock-protected).
func (s *Store) GetSettings() types.Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

// SaveSettings persists the given settings, normalising CDPPort and forcing
// SelectorConfigPath to its canonical (non-user-editable) value. Returns
// the canonical settings even if persistence fails.
func (s *Store) SaveSettings(in types.Settings) (types.Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if in.CDPPort <= 0 {
		in.CDPPort = 9222
	}
	in.SelectorConfigPath = s.selectorCfg
	s.settings = in
	if err := s.saveSettingsLocked(); err != nil {
		return s.settings, err
	}
	return s.settings, nil
}
