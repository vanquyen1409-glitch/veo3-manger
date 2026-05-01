package main

import (
	"fmt"
	"path/filepath"
	"time"

	"veo3-manager/internal/cdp"
	"veo3-manager/internal/types"
)

// cdpLaunchPollInterval and cdpLaunchTimeout control how aggressively we
// re-probe Chrome's CDP endpoint after spawning the process.
const (
	cdpLaunchPollInterval = 500 * time.Millisecond
	cdpLaunchTimeout      = 15 * time.Second
)

// GetCDPStatus returns the last-known CDP connection state from the
// controller without re-probing.
func (a *App) GetCDPStatus() types.CDPStatus {
	return a.cdp.Status()
}

// TestCDPConnection probes the CDP endpoint and emits a cdp:status event
// so any UI listeners are kept in sync.
func (a *App) TestCDPConnection() types.CDPStatus {
	settings := a.store.GetSettings()
	s := a.cdp.Probe(settings.CDPPort)
	a.emitCDP(s)
	return s
}

// LaunchChromeCDP starts a Chrome instance with the CDP debug port enabled,
// using the chromePath / cdpPort from settings and a profile directory under
// %APPDATA%/veo3-manager/chrome-profile. If Chrome is already responding on
// the port, returns the existing status without launching a new instance.
//
// Polls every 500ms for up to 15s to confirm the CDP server is responsive
// before returning success. Surfaces a clear error if Chrome was launched
// but never started serving CDP (e.g. port collision, Chrome crash).
func (a *App) LaunchChromeCDP() (types.CDPStatus, error) {
	settings := a.store.GetSettings()

	if s := a.cdp.Probe(settings.CDPPort); s.Connected {
		a.emitCDP(s)
		return s, nil
	}
	if settings.ChromePath == "" {
		return types.CDPStatus{}, fmt.Errorf("chưa cấu hình đường dẫn Chrome ở Cài Đặt — bấm Tự động phát hiện hoặc nhập tay")
	}

	profileDir := filepath.Join(a.store.Dir(), "chrome-profile")
	if err := cdp.LaunchChrome(settings.ChromePath, profileDir, settings.CDPPort); err != nil {
		return types.CDPStatus{}, err
	}

	last := a.waitForCDPReady(settings.CDPPort)
	a.emitCDP(last)
	if !last.Connected {
		return last, fmt.Errorf("Chrome đã khởi chạy nhưng không phản hồi CDP cổng %d trong %s — kiểm tra cổng có bị chiếm không", settings.CDPPort, cdpLaunchTimeout)
	}
	return last, nil
}

// waitForCDPReady polls the controller's Probe until it reports Connected or
// the launch timeout elapses. Returns the last status observed.
func (a *App) waitForCDPReady(port int) types.CDPStatus {
	deadline := time.Now().Add(cdpLaunchTimeout)
	var last types.CDPStatus
	for time.Now().Before(deadline) {
		time.Sleep(cdpLaunchPollInterval)
		last = a.cdp.Probe(port)
		if last.Connected {
			return last
		}
	}
	return last
}
