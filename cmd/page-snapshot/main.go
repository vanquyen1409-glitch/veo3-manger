// page-snapshot dumps current state: URL, any visible toasts/errors,
// any newly-rendered video tiles, and current submit button state.
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

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

const snapshotJS = `(() => {
  const out = {url: location.href, snapshot_at: new Date().toISOString()};
  // Slate state
  const editor = document.querySelector('[data-slate-editor="true"][role="textbox"]');
  out.slateExists = !!editor;
  if (editor) {
    out.slateInnerText = (editor.innerText || '').slice(0, 200);
    out.slateInnerHTML = (editor.innerHTML || '').slice(0, 500);
    out.placeholders = Array.from(editor.querySelectorAll('[data-slate-placeholder]')).map(p => (p.innerText||'').slice(0,40));
  }
  // Visible submit button state
  let btn = null;
  for (const b of document.querySelectorAll('button, [role="button"]')) {
    const r = b.getBoundingClientRect();
    if (r.y < window.innerHeight * 0.5 || r.width === 0) continue;
    const t = (b.innerText || '').trim();
    if (/arrow_forward/.test(t) && !/add_2/.test(t)) {
      if (!btn || r.x > btn._x) btn = {_x: r.x, text: t.slice(0,40), disabled: b.disabled || b.getAttribute('aria-disabled') === 'true', x: Math.round(r.x), y: Math.round(r.y)};
    }
  }
  out.submitBtn = btn;
  // Any toasts / aria-live regions
  out.aria = [];
  for (const a of document.querySelectorAll('[aria-live], [role="alert"], [role="status"]')) {
    const t = (a.innerText || '').trim();
    if (t) out.aria.push(t.slice(0,150));
  }
  // Visible error-ish text near bottom
  out.recentErrors = [];
  for (const el of document.querySelectorAll('div, span, p')) {
    const r = el.getBoundingClientRect();
    if (r.y < window.innerHeight * 0.3 || r.width === 0) continue;
    const t = (el.innerText || '').trim();
    if (/error|fail|lỗi|không|invalid/i.test(t) && t.length < 200) {
      out.recentErrors.push(t.slice(0,150));
    }
  }
  // Any video tiles
  const videos = document.querySelectorAll('video, [data-testid*="video"], [class*="video-tile"]');
  out.videoCount = videos.length;
  return JSON.stringify(out);
})()`

type cdpTarget struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	URL  string `json:"url"`
}

func main() {
	port := flag.Int("port", 9222, "CDP port")
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

	timeoutCtx, cancel := context.WithTimeout(tabCtx, 5*time.Second)
	defer cancel()

	var raw string
	if err := chromedp.Run(timeoutCtx, chromedp.Evaluate(snapshotJS, &raw)); err != nil {
		log.Fatal(err)
	}
	var pretty map[string]any
	_ = json.Unmarshal([]byte(raw), &pretty)
	out, _ := json.MarshalIndent(pretty, "", "  ")
	fmt.Println(string(out))
}
