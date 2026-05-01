package main

import (
	"errors"
	"os/exec"
	"runtime"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// SelectFolder opens the native folder-picker dialog and returns the absolute
// path the user chose. Returns "" when the user cancels (Wails reports cancel
// as an empty string + nil error).
func (a *App) SelectFolder() (string, error) {
	if a.ctx == nil {
		return "", errors.New("app context chưa sẵn sàng")
	}
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Chọn thư mục",
	})
}

// OpenPathInOS reveals the given file or folder in the native OS file
// manager: Explorer on Windows, Finder on macOS, xdg-open on Linux.
//
// The path is forwarded as a separate argv element to avoid shell-quoting
// issues. We don't validate that the path exists — the OS handler reports
// its own error to the user if it doesn't.
func (a *App) OpenPathInOS(path string) error {
	if path == "" {
		return errors.New("đường dẫn rỗng")
	}
	cmd, err := openPathCommand(path)
	if err != nil {
		return err
	}
	// Start (not Run) — fire-and-forget so the UI thread isn't blocked
	// waiting for Explorer to fully launch.
	return cmd.Start()
}

// openPathCommand builds the platform-specific command that opens path in the
// native file manager. Split out so the routing logic is unit-testable
// without actually spawning a process.
func openPathCommand(path string) (*exec.Cmd, error) {
	switch runtime.GOOS {
	case "windows":
		// `cmd /c start "" "<path>"` — empty "" is the window title slot
		// that `start` consumes; without it `start` would interpret the
		// path itself as a title when the path is quoted.
		return exec.Command("cmd", "/c", "start", "", path), nil
	case "darwin":
		return exec.Command("open", path), nil
	case "linux":
		return exec.Command("xdg-open", path), nil
	default:
		return nil, errors.New("hệ điều hành không được hỗ trợ: " + runtime.GOOS)
	}
}
