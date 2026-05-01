package cdp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"

	"veo3-manager/internal/types"
)

type Controller struct {
	connected atomic.Bool
	message   atomic.Value // string
}

func New() *Controller {
	c := &Controller{}
	c.message.Store("Chưa kết nối")
	return c
}

func (c *Controller) Status() types.CDPStatus {
	msg, _ := c.message.Load().(string)
	return types.CDPStatus{Connected: c.connected.Load(), Message: msg}
}

// Probe issues an HTTP GET to the Chrome CDP /json/version endpoint and
// updates internal state. Safe to call from multiple goroutines concurrently;
// last writer wins on the message field, which is acceptable since the result
// is also returned to the caller. No mutex held across the network call.
func (c *Controller) Probe(port int) types.CDPStatus {
	if port <= 0 {
		port = 9222
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/json/version", port)
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return c.setStatus(false, fmt.Sprintf("Không tìm thấy Chrome trên cổng %d", port))
	}
	defer resp.Body.Close()

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return c.setStatus(false, fmt.Sprintf("Phản hồi CDP không hợp lệ: %v", err))
	}

	browser, _ := body["Browser"].(string)
	if browser == "" {
		browser = "Chrome"
	}
	return c.setStatus(true, fmt.Sprintf("Đã kết nối %s", browser))
}

func (c *Controller) MarkDisconnected(reason string) {
	if reason == "" {
		reason = "Đã mất kết nối"
	}
	c.setStatus(false, reason)
}

// setStatus atomically updates internal state and returns the corresponding
// CDPStatus. Centralises the connected/message store-pair so callers don't
// have to keep them in sync.
func (c *Controller) setStatus(connected bool, msg string) types.CDPStatus {
	c.connected.Store(connected)
	c.message.Store(msg)
	return types.CDPStatus{Connected: connected, Message: msg}
}

// LaunchChrome spawns a Chrome process with --remote-debugging-port and
// --user-data-dir set, returning immediately. The caller is responsible for
// polling /json/version (e.g. via Controller.Probe) until the CDP server
// is up. The Chrome process runs independently of the parent app — closing
// the app does NOT close Chrome.
//
// Returns an error if chromePath is empty or doesn't exist as a file. The
// profileDir is created if missing (recursive mkdir).
func LaunchChrome(chromePath, profileDir string, port int) error {
	if chromePath == "" {
		return errors.New("đường dẫn Chrome rỗng")
	}
	info, err := os.Stat(chromePath)
	if err != nil {
		return fmt.Errorf("không tìm thấy Chrome tại %s: %w", chromePath, err)
	}
	if info.IsDir() {
		return fmt.Errorf("đường dẫn Chrome phải là file, không phải thư mục: %s", chromePath)
	}
	if profileDir == "" {
		return errors.New("profileDir rỗng")
	}
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return fmt.Errorf("tạo profile dir: %w", err)
	}
	if port <= 0 {
		port = 9222
	}
	cmd := exec.Command(chromePath,
		fmt.Sprintf("--remote-debugging-port=%d", port),
		fmt.Sprintf("--user-data-dir=%s", profileDir),
	)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("exec Chrome: %w", err)
	}
	// Detach: don't Wait. Chrome lives independently of this Go process.
	go func() {
		_ = cmd.Process.Release()
	}()
	return nil
}

func DetectChromePath() string {
	if runtime.GOOS != "windows" {
		return ""
	}
	candidates := []string{
		os.Getenv("ProgramFiles") + `\Google\Chrome\Application\chrome.exe`,
		os.Getenv("ProgramFiles(x86)") + `\Google\Chrome\Application\chrome.exe`,
		os.Getenv("LOCALAPPDATA") + `\Google\Chrome\Application\chrome.exe`,
		os.Getenv("ProgramFiles") + `\Microsoft\Edge\Application\msedge.exe`,
		os.Getenv("ProgramFiles(x86)") + `\Microsoft\Edge\Application\msedge.exe`,
	}
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return filepath.Clean(p)
		}
	}
	return ""
}
