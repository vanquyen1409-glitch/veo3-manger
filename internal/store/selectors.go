package store

import (
	"os"
)

// SelectorConfigPath returns the absolute path to selectors.json (the
// user-editable DOM-selector overrides file).
func (s *Store) SelectorConfigPath() string { return s.selectorCfg }

// ensureSelectorConfig writes a default selectors.json on first run, leaving
// any existing user file untouched. Defaults reflect current Google Flow DOM.
func (s *Store) ensureSelectorConfig() error {
	_, err := os.Stat(s.selectorCfg)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	defaults := map[string]string{
		"promptInput":  "[contenteditable=\"true\"][data-slate-editor=\"true\"]",
		"submitButton": "button[aria-label=\"Create\" i]",
		"aspectTab":    "[role=\"tab\"]",
		"settingsMenu": "button[aria-haspopup=\"menu\"]",
		"videoElement": "video",
		"downloadLink": "a[download]",
	}
	return writeJSONAtomic(s.selectorCfg, defaults)
}
