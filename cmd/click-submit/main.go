// click-submit dispatches a real mouse-click via CDP at the coordinates
// of the bottom-right submit button (arrow_forward), then drains
// __capRecaptcha for ~30s. Bypasses any flaky JS click handler issues.
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

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
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

const findSubmitJS = `(() => {
  // Find the rightmost arrow_forward button in lower half of viewport.
  let best = null;
  for (const b of document.querySelectorAll('button, [role="button"]')) {
    const r = b.getBoundingClientRect();
    if (r.y < window.innerHeight * 0.5 || r.width === 0) continue;
    const t = (b.innerText || '').trim();
    if (/arrow_forward/.test(t) && !/add_2/.test(t)) {
      if (!best || r.x > best.x) {
        best = {x: r.x, y: r.y, w: r.width, h: r.height};
      }
    }
  }
  return JSON.stringify(best || {err: 'no_btn'});
})()`

// focusEditorJS focuses the Slate editor and returns its bounding box so the
// caller can dispatch a real CDP click + Input.insertText (which Slate's
// beforeinput listener sees, unlike execCommand insertText).
const focusEditorJS = `(() => {
  const editor = document.querySelector('[data-slate-editor="true"][role="textbox"]');
  if (!editor) return JSON.stringify({err: 'no_editor'});
  editor.focus();
  const r = editor.getBoundingClientRect();
  return JSON.stringify({x: r.x, y: r.y, w: r.width, h: r.height});
})()`

const drainJS = `JSON.stringify((window.__capRecaptcha || []).splice(0))`

type cdpTarget struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	URL  string `json:"url"`
}

func awaitPromise(p *runtime.EvaluateParams) *runtime.EvaluateParams {
	return p.WithAwaitPromise(true)
}

func main() {
	port := flag.Int("port", 9222, "CDP port")
	prompt := flag.String("prompt", "a peaceful beach at sunset", "prompt to fill (or empty to skip)")
	wait := flag.Duration("wait", 60*time.Second, "drain duration after click")
	flag.Parse()

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json", *port))
	if err != nil {
		log.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var all []cdpTarget
	_ = json.Unmarshal(body, &all)
	var pageID string
	for _, t := range all {
		if t.Type == "page" && strings.Contains(t.URL, "/project/") {
			pageID = t.ID
			break
		}
	}
	if pageID == "" {
		log.Fatal("no labs.google /project/ tab")
	}

	parent := context.Background()
	allocCtx, _ := chromedp.NewRemoteAllocator(parent, fmt.Sprintf("http://127.0.0.1:%d", *port))
	browserCtx, _ := chromedp.NewContext(allocCtx)
	tabCtx, _ := chromedp.NewContext(browserCtx, chromedp.WithTargetID(target.ID(pageID)))

	// Re-install hook (no-op if already installed).
	_ = chromedp.Run(tabCtx, chromedp.ActionFunc(func(c context.Context) error {
		_, err := page.AddScriptToEvaluateOnNewDocument(installCaptureJS).WithRunImmediately(true).Do(c)
		return err
	}))
	_ = chromedp.Run(tabCtx, chromedp.Evaluate(installCaptureJS, nil))
	log.Println("hook (re)installed")

	if *prompt != "" {
		var focusRaw string
		if err := chromedp.Run(tabCtx, chromedp.Evaluate(focusEditorJS, &focusRaw)); err != nil {
			log.Fatalf("focus editor: %v", err)
		}
		var bbox struct {
			X, Y, W, H float64
			Err        string `json:"err"`
		}
		_ = json.Unmarshal([]byte(focusRaw), &bbox)
		if bbox.Err != "" || bbox.W == 0 {
			log.Fatalf("focus result: %s", focusRaw)
		}
		log.Printf("editor at (%.0f,%.0f), clicking + inserting via CDP", bbox.X, bbox.Y)
		// Click into editor at center to position caret.
		ex, ey := bbox.X+bbox.W/2, bbox.Y+bbox.H/2
		if err := chromedp.Run(tabCtx, chromedp.ActionFunc(func(c context.Context) error {
			if err := input.DispatchMouseEvent(input.MousePressed, ex, ey).WithButton(input.Left).WithClickCount(1).Do(c); err != nil {
				return err
			}
			time.Sleep(40 * time.Millisecond)
			return input.DispatchMouseEvent(input.MouseReleased, ex, ey).WithButton(input.Left).WithClickCount(1).Do(c)
		})); err != nil {
			log.Fatalf("editor click: %v", err)
		}
		time.Sleep(120 * time.Millisecond)
		// Native Input.insertText — fires beforeinput + input events Slate listens to.
		if err := chromedp.Run(tabCtx, chromedp.ActionFunc(func(c context.Context) error {
			return input.InsertText(*prompt).Do(c)
		})); err != nil {
			log.Fatalf("insertText: %v", err)
		}
		log.Printf("inserted prompt via CDP")
		time.Sleep(500 * time.Millisecond)
	}

	var btnRaw string
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(findSubmitJS, &btnRaw)); err != nil {
		log.Fatalf("find submit: %v", err)
	}
	var btn struct {
		X, Y, W, H float64
		Err        string `json:"err"`
	}
	_ = json.Unmarshal([]byte(btnRaw), &btn)
	if btn.Err != "" || btn.W == 0 {
		log.Fatalf("find submit returned: %s", btnRaw)
	}
	cx := btn.X + btn.W/2
	cy := btn.Y + btn.H/2
	log.Printf("submit btn at (%.0f,%.0f) — clicking center (%.0f,%.0f)", btn.X, btn.Y, cx, cy)

	// CDP-level mouse click: more reliable than JS click() for React/Radix UI.
	if err := chromedp.Run(tabCtx, chromedp.ActionFunc(func(c context.Context) error {
		if err := input.DispatchMouseEvent(input.MousePressed, cx, cy).
			WithButton(input.Left).
			WithClickCount(1).Do(c); err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
		return input.DispatchMouseEvent(input.MouseReleased, cx, cy).
			WithButton(input.Left).
			WithClickCount(1).Do(c)
	})); err != nil {
		log.Fatalf("CDP click: %v", err)
	}
	log.Println("click dispatched")

	// Drain loop.
	deadline := time.Now().Add(*wait)
	for time.Now().Before(deadline) {
		var raw string
		if err := chromedp.Run(tabCtx, chromedp.Evaluate(drainJS, &raw)); err != nil {
			log.Printf("drain: %v", err)
		} else {
			raw = strings.TrimSpace(raw)
			if raw != "" && raw != "null" && raw != "[]" {
				var arr []map[string]any
				if json.Unmarshal([]byte(raw), &arr) == nil {
					for _, c := range arr {
						fmt.Printf("=== CAPTURED siteKey=%v action=%v t=%v ===\n", c["siteKey"], c["action"], c["t"])
					}
				}
			}
		}
		time.Sleep(700 * time.Millisecond)
	}
	log.Println("drain window closed")
}
