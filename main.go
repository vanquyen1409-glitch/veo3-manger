package main

import (
	"embed"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// localFileHandler serves files from the configured output directory under
// the URL prefix /localfile/. Requests outside the configured directory are
// rejected with 403 to prevent path traversal attacks.
func localFileHandler(app *App) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "/localfile/"
		if !strings.HasPrefix(r.URL.Path, prefix) {
			http.NotFound(w, r)
			return
		}
		raw := strings.TrimPrefix(r.URL.Path, prefix)
		decoded, err := url.PathUnescape(raw)
		if err != nil {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		requested, err := filepath.Abs(filepath.Clean(decoded))
		if err != nil {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}

		settings := app.store.GetSettings()
		root, err := filepath.Abs(filepath.Clean(settings.OutputDir))
		if err != nil || root == "" {
			http.Error(w, "output dir not configured", http.StatusInternalServerError)
			return
		}

		// Ensure the requested path is contained within root.
		rel, err := filepath.Rel(root, requested)
		if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		http.ServeFile(w, r, requested)
	})
}

func main() {
	app, err := NewApp()
	if err != nil {
		log.Fatalf("init app: %v", err)
	}

	err = wails.Run(&options.App{
		Title:     "Veo3 Manager",
		Width:     1280,
		Height:    820,
		MinWidth:  1024,
		MinHeight: 700,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: localFileHandler(app),
		},
		BackgroundColour: &options.RGBA{R: 243, G: 244, B: 246, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatalf("wails run: %v", err)
	}
}
