// learn-action is the Approach D discovery tool. It connects to the running
// Chrome on :9222, finds the labs.google Flow tab, installs a hook on
// grecaptcha.enterprise.execute, then waits for the user to click "Tạo Video"
// in the page. When the page calls grecaptcha, we capture the (siteKey,
// action) pair and write `learnedRecaptchaAction` to %APPDATA%/veo3-manager/
// settings.json so all subsequent submits via the desktop app reuse it.
//
// Usage (after Chrome is up with --remote-debugging-port=9222 + signed in):
//
//	go run ./cmd/learn-action            # waits up to 5 minutes for capture
//	go run ./cmd/learn-action -wait 10m  # longer window
//	go run ./cmd/learn-action -force     # overwrite an existing learned action
//
// You only need to run this ONCE per Google Labs API revision. The action
// rarely changes; if a future API rev breaks it, re-run.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

const installCaptureJS = `
(() => {
  if (window.__capRecaptchaInstalled) return;
  window.__capRecaptchaInstalled = true;
  if (!Array.isArray(window.__capRecaptcha)) window.__capRecaptcha = [];
  const wrap = () => {
    try {
      if (!(window.grecaptcha && window.grecaptcha.enterprise && window.grecaptcha.enterprise.execute)) {
        setTimeout(wrap, 100); return;
      }
      if (window.grecaptcha.enterprise.__capWrapped) return;
      window.grecaptcha.enterprise.__capWrapped = true;
      const orig = window.grecaptcha.enterprise.execute;
      window.grecaptcha.enterprise.execute = function(siteKey, options) {
        try {
          const action = options && typeof options === 'object' ? (options.action || '') : '';
          window.__capRecaptcha.push({ siteKey: String(siteKey || ''), action: String(action), t: Date.now() });
        } catch (_) {}
        return orig.apply(this, arguments);
      };
    } catch (_) { setTimeout(wrap, 200); }
  };
  wrap();
})()
`

const drainJS = `JSON.stringify((window.__capRecaptcha || []).splice(0))`

type cdpTarget struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	URL  string `json:"url"`
}

type capturedCall struct {
	SiteKey string `json:"siteKey"`
	Action  string `json:"action"`
	T       int64  `json:"t"`
}

// settingsShape mirrors internal/types.Settings but inline to avoid a Wails
// dependency just for reading/writing JSON.
type settingsShape struct {
	ChromePath             string `json:"chromePath"`
	CDPPort                int    `json:"cdpPort"`
	SelectorConfigPath     string `json:"selectorConfigPath"`
	OutputDir              string `json:"outputDir"`
	LearnedRecaptchaAction string `json:"learnedRecaptchaAction,omitempty"`
}

func settingsPath() string {
	return filepath.Join(os.Getenv("APPDATA"), "veo3-manager", "settings.json")
}

func loadSettings() (settingsShape, error) {
	var s settingsShape
	data, err := os.ReadFile(settingsPath())
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, err
	}
	return s, nil
}

func saveSettings(s settingsShape) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath(), data, 0o644)
}

func findLabsTab(port int) (*cdpTarget, error) {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json", port))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var all []cdpTarget
	if err := json.Unmarshal(body, &all); err != nil {
		return nil, err
	}
	// Prefer /project/ tabs, fall back to any labs.google page.
	for _, t := range all {
		if t.Type == "page" && strings.Contains(t.URL, "/project/") {
			tt := t
			return &tt, nil
		}
	}
	for _, t := range all {
		if t.Type == "page" && strings.Contains(t.URL, "labs.google") {
			tt := t
			return &tt, nil
		}
	}
	return nil, fmt.Errorf("không tìm thấy tab labs.google nào trên CDP cổng %d — mở https://labs.google/fx/tools/flow trong Chrome đó trước đã", port)
}

func main() {
	port := flag.Int("port", 9222, "CDP port")
	wait := flag.Duration("wait", 5*time.Minute, "max time to wait for the user to click Tạo Video")
	force := flag.Bool("force", false, "overwrite existing learned action even if already set")
	dryRun := flag.Bool("dry-run", false, "print captured action but don't save to settings.json")
	flag.Parse()

	existing, _ := loadSettings()
	if existing.LearnedRecaptchaAction != "" && !*force && !*dryRun {
		log.Printf("settings.json đã có learnedRecaptchaAction=%q — dùng -force để ghi đè hoặc -dry-run để chỉ thử", existing.LearnedRecaptchaAction)
		return
	}

	tab, err := findLabsTab(*port)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("attached: tab id=%s url=%s", tab.ID, tab.URL)

	parent := context.Background()
	allocCtx, _ := chromedp.NewRemoteAllocator(parent, fmt.Sprintf("http://127.0.0.1:%d", *port))
	browserCtx, _ := chromedp.NewContext(allocCtx)
	tabCtx, _ := chromedp.NewContext(browserCtx, chromedp.WithTargetID(target.ID(tab.ID)))

	// Register on new docs (covers any in-page navigation).
	if err := chromedp.Run(tabCtx, chromedp.ActionFunc(func(c context.Context) error {
		_, err := page.AddScriptToEvaluateOnNewDocument(installCaptureJS).
			WithRunImmediately(true).
			Do(c)
		return err
	})); err != nil {
		log.Fatalf("addScript: %v", err)
	}
	// Also evaluate immediately on the current document (the script
	// above only runs on NEW docs; the user is already on a doc).
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(installCaptureJS, nil)); err != nil {
		log.Fatalf("evaluate install: %v", err)
	}
	log.Printf("hook installed on grecaptcha.enterprise.execute")

	fmt.Println()
	fmt.Println("================================================================")
	fmt.Println(" Bây giờ HÃY CLICK 'Tạo Video' trong tab Chrome đang mở.")
	fmt.Println(" Đảm bảo: model là Veo (KHÔNG phải Nano Banana), có prompt,")
	fmt.Println(" cấu hình aspect/duration. Click Tạo và chờ — submit thật")
	fmt.Println(" sẽ chạy nhưng KHÔNG cần đợi tới hoàn thành.")
	fmt.Printf(" Đang chờ tối đa %s...\n", *wait)
	fmt.Println("================================================================")
	fmt.Println()

	deadline := time.Now().Add(*wait)
	for time.Now().Before(deadline) {
		var raw string
		if err := chromedp.Run(tabCtx, chromedp.Evaluate(drainJS, &raw)); err != nil {
			log.Printf("drain: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}
		raw = strings.TrimSpace(raw)
		if raw != "" && raw != "null" && raw != "[]" {
			var arr []capturedCall
			if err := json.Unmarshal([]byte(raw), &arr); err != nil {
				log.Printf("parse capture: %v", err)
				continue
			}
			if len(arr) == 0 {
				continue
			}
			// Prefer the LATEST entry — that's the most recent click.
			best := arr[len(arr)-1]
			fmt.Printf("\n=== CAPTURED ===\n  siteKey: %s\n  action:  %q\n  t:       %d\n\n", best.SiteKey, best.Action, best.T)
			if *dryRun {
				fmt.Println("(-dry-run: không lưu)")
				return
			}
			existing.LearnedRecaptchaAction = best.Action
			if err := saveSettings(existing); err != nil {
				log.Fatalf("save settings: %v", err)
			}
			fmt.Printf("Đã lưu vào %s\n", settingsPath())
			fmt.Println("Lần chạy app sau sẽ dùng action này tự động.")
			return
		}
		time.Sleep(700 * time.Millisecond)
	}
	log.Fatalf("hết thời gian chờ %s mà chưa thấy grecaptcha call — bạn đã click Tạo Video chưa?", *wait)
}
