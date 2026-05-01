package cdp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLaunchChrome_EmptyPathRejected(t *testing.T) {
	err := LaunchChrome("", t.TempDir(), 9222)
	if err == nil {
		t.Fatal("expected error for empty chromePath")
	}
	if !strings.Contains(err.Error(), "rỗng") {
		t.Errorf("error should mention empty: %v", err)
	}
}

func TestLaunchChrome_NonexistentBinaryRejected(t *testing.T) {
	err := LaunchChrome(filepath.Join(t.TempDir(), "no-such-chrome.exe"), t.TempDir(), 9222)
	if err == nil {
		t.Fatal("expected error for nonexistent binary")
	}
	if !strings.Contains(err.Error(), "không tìm thấy") {
		t.Errorf("error should mention not found: %v", err)
	}
}

func TestLaunchChrome_DirectoryNotFile(t *testing.T) {
	dir := t.TempDir()
	err := LaunchChrome(dir, t.TempDir(), 9222)
	if err == nil {
		t.Fatal("expected error when chromePath points to a directory")
	}
	if !strings.Contains(err.Error(), "thư mục") {
		t.Errorf("error should mention it's a dir: %v", err)
	}
}

func TestLaunchChrome_EmptyProfileDirRejected(t *testing.T) {
	dir := t.TempDir()
	fakeChrome := filepath.Join(dir, "fake-chrome.exe")
	if err := os.WriteFile(fakeChrome, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := LaunchChrome(fakeChrome, "", 9222)
	if err == nil {
		t.Fatal("expected error for empty profileDir")
	}
}

func TestController_StatusInitiallyDisconnected(t *testing.T) {
	c := New()
	s := c.Status()
	if s.Connected {
		t.Error("freshly constructed controller should be disconnected")
	}
	if s.Message == "" {
		t.Error("freshly constructed controller should have a message")
	}
}

func TestController_MarkDisconnected(t *testing.T) {
	c := New()
	c.connected.Store(true)
	c.MarkDisconnected("test reason")
	s := c.Status()
	if s.Connected {
		t.Error("MarkDisconnected should set connected=false")
	}
	if s.Message != "test reason" {
		t.Errorf("message not updated: %q", s.Message)
	}
}

func TestController_MarkDisconnectedDefaultMessage(t *testing.T) {
	c := New()
	c.connected.Store(true)
	c.MarkDisconnected("")
	if msg := c.Status().Message; msg == "" {
		t.Error("empty reason should produce a default non-empty message")
	}
}

func TestProbe_NoChromeRunning(t *testing.T) {
	// Use a port that's almost certainly not in use.
	c := New()
	s := c.Probe(1) // port 1 typically requires root and is rarely bound
	if s.Connected {
		t.Error("Probe should fail when no CDP server is listening")
	}
	if s.Message == "" {
		t.Error("Probe should set a failure message")
	}
}
