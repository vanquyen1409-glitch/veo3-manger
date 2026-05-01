package main

import (
	"veo3-manager/internal/cdp"
	"veo3-manager/internal/types"
)

// GetSettings returns the current persisted settings. Bound to JS as
// GetSettings().
func (a *App) GetSettings() types.Settings {
	return a.store.GetSettings()
}

// SaveSettings persists the given settings and returns the canonical form
// (with selectorConfigPath re-injected and CDPPort defaulted to 9222 if zero).
func (a *App) SaveSettings(s types.Settings) (types.Settings, error) {
	return a.store.SaveSettings(s)
}

// DetectChromePath returns the first existing Chrome/Edge binary on disk in
// the common Windows install locations, or "" if nothing was found.
func (a *App) DetectChromePath() string {
	return cdp.DetectChromePath()
}

// GetSelectorConfigPath returns the absolute path to the selector overrides
// JSON file (canonical, not user-editable via Settings).
func (a *App) GetSelectorConfigPath() string {
	return a.store.SelectorConfigPath()
}
