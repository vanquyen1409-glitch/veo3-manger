package store

import (
	"encoding/json"
	"fmt"
	"os"
)

// writeJSONAtomic marshals v with indentation and writes it to path through
// a temp file + rename. This guarantees that a reader either sees the prior
// contents or the fully-written new contents — never a half-written file
// from a crash mid-write.
//
// On success the temp file is gone. On failure the destination is untouched
// and the temp file is removed best-effort. The 0644 mode matches the
// previous direct-WriteFile behaviour.
func writeJSONAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename tmp → %s: %w", path, err)
	}
	return nil
}
