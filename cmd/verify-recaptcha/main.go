// verify-recaptcha attaches to the existing labs.google tab on Chrome :9222,
// installs the new capture hook from internal/automation, waits, then drains
// to show any (siteKey, action) pairs the page itself called grecaptcha
// with. This verifies the fix without driving a real submit (no credit cost).
//
// Usage: go run ./cmd/verify-recaptcha -wait 8s
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// installCaptureJS is a verbatim copy of the const in
// internal/automation/recaptcha.go; we duplicate it here so this verifier
// has zero dependency on the package's chromedp browser type (it attaches
// to a specific tab via target ID, not via the package's Connect path).
const installCaptureJS = `
(() => {
  if (window.__capRecaptchaInstalled) return;
  window.__capRecaptchaInstalled = true;
  if (!Array.isArray(window.__capRecaptcha)) window.__capRecaptcha = [];
  const wrap = () => {
    try {
      if (!(window.grecaptcha && window.grecaptcha.enterprise && window.grecaptcha.enterprise.execute)) {
        setTimeout(wrap, 100);
        return;
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
    } catch (_) {
      setTimeout(wrap, 200);
    }
  };
  wrap();
})()
`

const drainJS = `JSON.stringify((window.__capRecaptcha || []).splice(0))`

type cdpTarget struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

func findFlowTab(port int) (*cdpTarget, error) {
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
	return nil, fmt.Errorf("không tìm thấy tab labs.google")
}

func main() {
	port := flag.Int("port", 9222, "CDP port")
	wait := flag.Duration("wait", 8*time.Second, "how long to wait for the page to call grecaptcha after hook install")
	flag.Parse()

	tab, err := findFlowTab(*port)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("attached to tab id=%s url=%s", tab.ID, tab.URL)

	parent := context.Background()
	allocCtx, _ := chromedp.NewRemoteAllocator(parent, fmt.Sprintf("http://127.0.0.1:%d", *port))
	browserCtx, _ := chromedp.NewContext(allocCtx)
	tabCtx, _ := chromedp.NewContext(browserCtx, chromedp.WithTargetID(target.ID(tab.ID)))

	// Install the hook BOTH ways: (1) on new documents (so future navigations
	// are covered) and (2) immediately on the current document via Evaluate
	// (so the live page is wrapped without requiring reload).
	if err := chromedp.Run(tabCtx, chromedp.ActionFunc(func(c context.Context) error {
		_, err := page.AddScriptToEvaluateOnNewDocument(installCaptureJS).
			WithRunImmediately(true).
			Do(c)
		return err
	})); err != nil {
		log.Fatalf("addScript: %v", err)
	}
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(installCaptureJS, nil)); err != nil {
		log.Fatalf("evaluate install: %v", err)
	}
	log.Printf("capture hook installed; waiting %s for grecaptcha calls...", *wait)

	// Drain repeatedly so we don't miss anything that fires while we wait.
	deadline := time.Now().Add(*wait)
	allCaps := []map[string]any{}
	for time.Now().Before(deadline) {
		var raw string
		if err := chromedp.Run(tabCtx, chromedp.Evaluate(drainJS, &raw)); err != nil {
			log.Printf("drain: %v", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		raw = strings.TrimSpace(raw)
		if raw != "" && raw != "null" && raw != "[]" {
			var arr []map[string]any
			if err := json.Unmarshal([]byte(raw), &arr); err == nil {
				allCaps = append(allCaps, arr...)
				for _, c := range arr {
					fmt.Printf("=== CAPTURED: siteKey=%v action=%v t=%v ===\n", c["siteKey"], c["action"], c["t"])
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	if len(allCaps) == 0 {
		log.Println("WARN: không có grecaptcha call nào trong cửa sổ chờ. Page có thể chưa gọi grecaptcha — thử click 'Tạo Video' trong Chrome rồi chạy lại verify.")
		return
	}

	// Print latest siteKey + action for easy copying.
	last := allCaps[len(allCaps)-1]
	fmt.Println("---")
	fmt.Printf("LATEST: siteKey=%v action=%v\n", last["siteKey"], last["action"])
	fmt.Printf("This is the action string the app should send via clientContext.recaptchaAction.\n")
}
