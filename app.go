// Package main hosts the App struct exposed to Wails and the Wails-bound
// methods. Method implementations are split across app_*.go files by domain:
//   - app_settings.go — Settings get/save + Chrome path detection
//   - app_cdp.go      — CDP probe / test / launch
//   - app_videos.go   — Video CRUD bindings (List/Create/Delete + helpers)
//   - app_pipeline.go — runGeneration multi-stage pipeline
//   - app_dialogs.go  — Native folder dialog + OpenPathInOS shell helper
package main

import (
	"context"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"veo3-manager/internal/cdp"
	"veo3-manager/internal/store"
	"veo3-manager/internal/types"
)

type App struct {
	ctx   context.Context
	store *store.Store
	cdp   *cdp.Controller
}

func NewApp() (*App, error) {
	st, err := store.New()
	if err != nil {
		return nil, err
	}
	return &App{
		store: st,
		cdp:   cdp.New(),
	}, nil
}

// startup is invoked by Wails after window creation. It captures the
// runtime context and kicks off an initial CDP probe so the sidebar
// indicator is accurate on launch.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	go func() {
		settings := a.store.GetSettings()
		s := a.cdp.Probe(settings.CDPPort)
		a.emitCDP(s)
	}()
}

func (a *App) emitCDP(s types.CDPStatus) {
	if a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, "cdp:status", s)
}

func (a *App) emitProgress(p types.ProgressEvent) {
	if a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, "video:progress", p)
}

func (a *App) emitVideosChanged() {
	if a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, "videos:changed")
}
