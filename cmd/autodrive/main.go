// autodrive opens the labs.google config panel, switches to Video tab,
// picks a Veo model from the inner dropdown, fills prompt, clicks submit,
// then drains captured fetch + grecaptcha events.
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

const installInterceptorJS = `
(() => {
  if (window.__capInstalled) return 'already-installed';
  window.__capInstalled = true;
  window.__capRequests = [];
  window.__capRecaptcha = [];
  const want = (u) => u && u.includes('aisandbox-pa.googleapis.com')
    && !u.includes('batchLog') && !u.includes('/credits') && !u.includes('flowWorkflows/');
  const origFetch = window.fetch;
  window.fetch = async function(input, init) {
    try {
      const url = typeof input === 'string' ? input : (input && input.url) || '';
      if (want(url)) {
        const method = (init && init.method) || (typeof input === 'object' && input && input.method) || 'GET';
        let headers = {};
        if (init && init.headers) {
          if (init.headers instanceof Headers) init.headers.forEach((v, k) => { headers[k] = v; });
          else if (Array.isArray(init.headers)) for (const [k, v] of init.headers) headers[k] = v;
          else headers = Object.assign({}, init.headers);
        }
        let body = (init && init.body) || null;
        if (body && typeof body !== 'string') { try { body = String(body); } catch(_) {} }
        window.__capRequests.push({ url, method, headers, body, t: Date.now(), via: 'fetch' });
      }
    } catch(e) {}
    return origFetch.apply(this, arguments);
  };
  const installGre = () => {
    if (!(window.grecaptcha && window.grecaptcha.enterprise && window.grecaptcha.enterprise.execute)) {
      setTimeout(installGre, 100);
      return;
    }
    const orig = window.grecaptcha.enterprise.execute;
    window.grecaptcha.enterprise.execute = function(siteKey, options) {
      try { window.__capRecaptcha.push({ siteKey, options, t: Date.now() }); } catch(e) {}
      return orig.apply(this, arguments);
    };
  };
  installGre();
  return 'installed';
})()
`

// findElementJS returns center coords + state for a single element matched
// by a CSS selector. Used for click targeting.
const findElementByIDJS = `
(() => {
  const el = document.getElementById(%q);
  if (!el) return JSON.stringify({err: 'not_found'});
  const r = el.getBoundingClientRect();
  return JSON.stringify({
    cx: Math.round(r.x + r.width/2), cy: Math.round(r.y + r.height/2),
    text: (el.innerText || '').replace(/\s+/g,' ').trim().slice(0,80),
    dataState: el.getAttribute('data-state') || '',
    ariaExpanded: el.getAttribute('aria-expanded') || '',
    visible: r.width > 0 && r.height > 0,
  });
})()
`

// findModelChipJS returns the OUTER bottom-bar config chip that opens the
// settings panel.
const findModelChipJS = `
(() => {
  for (const b of document.querySelectorAll('button[id^="radix-"][aria-haspopup="menu"]')) {
    const r = b.getBoundingClientRect();
    if (r.y < window.innerHeight * 0.6) continue;
    const t = (b.innerText || '').toLowerCase();
    if (/banana|veo|nano|imagen|wan|kling/i.test(t)) {
      return JSON.stringify({
        id: b.id, cx: Math.round(r.x + r.width/2), cy: Math.round(r.y + r.height/2),
        dataState: b.getAttribute('data-state') || '',
        ariaExpanded: b.getAttribute('aria-expanded') || '',
        text: (b.innerText || '').replace(/\s+/g, ' ').trim().slice(0, 100),
      });
    }
  }
  return JSON.stringify({err: 'chip_not_found'});
})()
`

// findVideoTabJS finds the Video tab trigger in the open config panel.
// IDs look like radix-:r3v:-trigger-VIDEO.
const findVideoTabJS = `
(() => {
  for (const t of document.querySelectorAll('button[id*="-trigger-VIDEO"]')) {
    const r = t.getBoundingClientRect();
    if (r.width === 0) continue;
    return JSON.stringify({
      id: t.id, cx: Math.round(r.x + r.width/2), cy: Math.round(r.y + r.height/2),
      dataState: t.getAttribute('data-state') || '',
    });
  }
  return JSON.stringify({err: 'video_tab_not_found'});
})()
`

// findInnerModelDropdownJS finds the inner model picker (the row that
// shows current model name + arrow_drop_down). Match by aria-haspopup=menu
// inside the open config panel and y > 540.
const findInnerModelDropdownJS = `
(() => {
  for (const b of document.querySelectorAll('button[id^="radix-"][aria-haspopup="menu"]')) {
    const r = b.getBoundingClientRect();
    if (r.width < 100) continue;
    if (r.y < 540) continue;
    const t = (b.innerText || '').toLowerCase();
    if (/arrow_drop_down/.test(t)) {
      return JSON.stringify({
        id: b.id, cx: Math.round(r.x + r.width/2), cy: Math.round(r.y + r.height/2),
        text: (b.innerText || '').replace(/\s+/g,' ').trim().slice(0,100),
        dataState: b.getAttribute('data-state') || '',
      });
    }
  }
  return JSON.stringify({err: 'inner_dropdown_not_found'});
})()
`

// findVeoMenuItemJS scans menuitems in any open Radix menu portal for one
// containing "veo" in its text.
const findVeoMenuItemJS = `
(() => {
  const items = [];
  for (const el of document.querySelectorAll('[role="menuitem"], [role="menuitemradio"], [role="option"]')) {
    const r = el.getBoundingClientRect();
    if (r.width === 0 || r.height === 0) continue;
    items.push({
      text: (el.innerText || '').replace(/\s+/g,' ').trim().slice(0,120),
      cx: Math.round(r.x + r.width/2), cy: Math.round(r.y + r.height/2),
    });
  }
  for (const it of items) {
    if (/veo/i.test(it.text)) {
      return JSON.stringify({pick: it, all: items});
    }
  }
  return JSON.stringify({err: 'no_veo_item', all: items});
})()
`

const findSubmitJS = `
(() => {
  const out = [];
  for (const b of document.querySelectorAll('button, [role="button"]')) {
    const r = b.getBoundingClientRect();
    if (r.y < window.innerHeight * 0.6 || r.width === 0) continue;
    const t = (b.innerText || '').trim();
    if (/arrow_forward/.test(t) && !/add_2/.test(t)) {
      out.push({
        cx: Math.round(r.x + r.width/2), cy: Math.round(r.y + r.height/2),
        disabled: b.disabled || b.getAttribute('aria-disabled') === 'true',
        text: t.slice(0, 60),
      });
    }
  }
  out.sort((a, b) => b.cx - a.cx);
  return JSON.stringify({btn: out[0] || null});
})()
`

const fillPromptJS = `
(async () => {
  const sleep = (ms) => new Promise(r => setTimeout(r, ms));
  const editor = document.querySelector('[data-slate-editor="true"][role="textbox"]');
  if (!editor) return JSON.stringify({err: 'editor_not_found'});
  editor.focus();
  await sleep(80);
  document.execCommand('selectAll', false);
  document.execCommand('delete', false);
  await sleep(40);
  document.execCommand('insertText', false, %q);
  await sleep(150);
  return JSON.stringify({status: 'filled', text: (editor.innerText || '').slice(0, 100)});
})()
`

const drainJS = `JSON.stringify({reqs: (window.__capRequests || []).splice(0), rec: (window.__capRecaptcha || []).splice(0)})`

type cdpTarget struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
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
	return nil, fmt.Errorf("no labs.google tab")
}

func click(ctx context.Context, x, y int) error {
	return chromedp.Run(ctx, chromedp.MouseClickXY(float64(x), float64(y)))
}

func eval(ctx context.Context, js string) (string, error) {
	var raw string
	err := chromedp.Run(ctx, chromedp.Evaluate(js, &raw))
	return raw, err
}

type elemInfo struct {
	ID, Text, DataState, AriaExpanded, Err string
	Cx, Cy                                 int
	Visible                                bool
}

func find(ctx context.Context, label, js string) (elemInfo, error) {
	r, err := eval(ctx, js)
	if err != nil {
		return elemInfo{}, err
	}
	var info elemInfo
	_ = json.Unmarshal([]byte(r), &info)
	log.Printf("[%s] %s", label, r)
	if info.Err != "" {
		return info, fmt.Errorf("%s: %s", label, info.Err)
	}
	return info, nil
}

func main() {
	port := flag.Int("port", 9222, "CDP port")
	prompt := flag.String("prompt", "test cat dancing in space", "prompt to insert")
	captureWindow := flag.Duration("capture", 30*time.Second, "drain window after submit")
	flag.Parse()

	tab, err := findLabsTab(*port)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("attached: %s url=%s", tab.ID, tab.URL)

	parent := context.Background()
	allocCtx, _ := chromedp.NewRemoteAllocator(parent, fmt.Sprintf("http://127.0.0.1:%d", *port))
	browserCtx, _ := chromedp.NewContext(allocCtx)
	tabCtx, _ := chromedp.NewContext(browserCtx, chromedp.WithTargetID(target.ID(tab.ID)))

	if r, err := eval(tabCtx, installInterceptorJS); err != nil {
		log.Fatalf("install: %v", err)
	} else {
		log.Printf("interceptor: %s", r)
	}

	// Step 1: Ensure config panel is OPEN. Click chip if state=closed.
	chip, err := find(tabCtx, "chip", findModelChipJS)
	if err != nil {
		log.Fatalf("find chip: %v", err)
	}
	if chip.DataState != "open" {
		log.Println("opening config panel...")
		if err := click(tabCtx, chip.Cx, chip.Cy); err != nil {
			log.Fatalf("click chip: %v", err)
		}
		time.Sleep(800 * time.Millisecond)
	}

	// Step 2: Click Video tab to switch from Image to Video mode.
	video, err := find(tabCtx, "video tab", findVideoTabJS)
	if err != nil {
		log.Fatalf("find video tab: %v", err)
	}
	if video.DataState != "active" {
		log.Println("clicking Video tab...")
		if err := click(tabCtx, video.Cx, video.Cy); err != nil {
			log.Fatalf("click video tab: %v", err)
		}
		time.Sleep(800 * time.Millisecond)
	}

	// Step 3: Click inner model dropdown to open Veo options.
	innerDD, err := find(tabCtx, "inner model dropdown", findInnerModelDropdownJS)
	if err != nil {
		log.Fatalf("find inner DD: %v", err)
	}
	if !strings.Contains(strings.ToLower(innerDD.Text), "veo") {
		log.Println("opening inner model dropdown...")
		if err := click(tabCtx, innerDD.Cx, innerDD.Cy); err != nil {
			log.Fatalf("click inner DD: %v", err)
		}
		time.Sleep(800 * time.Millisecond)

		// Step 4: Pick Veo from menu.
		r, err := eval(tabCtx, findVeoMenuItemJS)
		if err != nil {
			log.Fatalf("find veo: %v", err)
		}
		log.Printf("[veo pick] %s", r)
		var veo struct {
			Pick *struct {
				Text   string `json:"text"`
				Cx, Cy int
			} `json:"pick"`
			All []map[string]any `json:"all"`
			Err string           `json:"err"`
		}
		_ = json.Unmarshal([]byte(r), &veo)
		if veo.Pick == nil {
			log.Fatalf("no Veo found — items=%d", len(veo.All))
		}
		log.Printf("clicking Veo: %q at %d,%d", veo.Pick.Text, veo.Pick.Cx, veo.Pick.Cy)
		if err := click(tabCtx, veo.Pick.Cx, veo.Pick.Cy); err != nil {
			log.Fatalf("click veo: %v", err)
		}
		time.Sleep(1200 * time.Millisecond)
	} else {
		log.Println("inner DD already showing Veo, skipping")
	}

	// Step 5: Close config panel by clicking chip again, so submit button is unobstructed.
	// Actually safer: press Escape.
	log.Println("closing config panel via Escape...")
	if err := chromedp.Run(tabCtx, chromedp.KeyEvent("")); err != nil {
		log.Printf("escape: %v", err)
	}
	time.Sleep(400 * time.Millisecond)

	// Step 6: Fill prompt.
	jsFill := fmt.Sprintf(fillPromptJS, *prompt)
	if r, err := eval(tabCtx, jsFill); err != nil {
		log.Fatalf("fill: %v", err)
	} else {
		log.Printf("fill: %s", r)
	}
	time.Sleep(400 * time.Millisecond)

	// Step 7: Click submit.
	r, err := eval(tabCtx, findSubmitJS)
	if err != nil {
		log.Fatalf("find submit: %v", err)
	}
	log.Printf("submit: %s", r)
	var sub struct {
		Btn *struct {
			Cx, Cy   int
			Disabled bool
			Text     string
		} `json:"btn"`
	}
	_ = json.Unmarshal([]byte(r), &sub)
	if sub.Btn == nil {
		log.Fatal("submit button not found")
	}
	if sub.Btn.Disabled {
		log.Fatalf("submit DISABLED — text=%s", sub.Btn.Text)
	}
	log.Printf("clicking submit at %d,%d", sub.Btn.Cx, sub.Btn.Cy)
	if err := click(tabCtx, sub.Btn.Cx, sub.Btn.Cy); err != nil {
		log.Fatalf("click submit: %v", err)
	}

	// Step 8: Drain.
	deadline := time.Now().Add(*captureWindow)
	captured, gotRec := false, false
	for time.Now().Before(deadline) {
		time.Sleep(1 * time.Second)
		raw, err := eval(tabCtx, drainJS)
		if err != nil {
			continue
		}
		var p struct {
			Reqs []map[string]any `json:"reqs"`
			Rec  []map[string]any `json:"rec"`
		}
		_ = json.Unmarshal([]byte(raw), &p)
		if len(p.Reqs) == 0 && len(p.Rec) == 0 {
			continue
		}
		fmt.Println("=== drain tick ===")
		pretty, _ := json.MarshalIndent(p, "", "  ")
		fmt.Println(string(pretty))
		for _, r := range p.Reqs {
			if u, _ := r["url"].(string); strings.Contains(u, "batchAsync") || strings.Contains(u, "Generate") {
				fmt.Println("=== TARGET CAPTURED ===")
				captured = true
			}
		}
		if len(p.Rec) > 0 {
			fmt.Println("=== reCAPTCHA CAPTURED ===")
			gotRec = true
		}
		if captured && gotRec {
			return
		}
	}
	if !captured {
		log.Println("WARN: hết capture window, không thấy submit request")
	}
}
