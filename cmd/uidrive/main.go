// uidrive is a single-process driver that:
//  1. attaches to the running Chrome on port 9222,
//  2. ensures we're on a labs.google /project/ page,
//  3. opens the model dropdown, picks the first Veo menu item,
//  4. fills the Slate prompt editor,
//  5. installs a fetch interceptor (to capture the real submit request),
//  6. clicks the submit button,
//  7. polls the interceptor + watches for video card / signed-URL events.
//
// Everything runs in ONE Go process to avoid chromedp's per-context cleanup
// closing the labs.google tab between steps.
//
// Run: go run ./cmd/uidrive -prompt "a cat dancing"
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

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// installInterceptorJS hooks fetch + XHR + grecaptcha.execute. Captures only
// requests to aisandbox-pa.googleapis.com that are NOT analytics/credits.
const installInterceptorJS = `
(() => {
  if (window.__capInstalled) return 'already-installed';
  window.__capInstalled = true;
  window.__capRequests = [];
  window.__capRecaptcha = [];

  const want = (u) => u && u.includes('aisandbox-pa.googleapis.com')
    && !u.includes('batchLog')
    && !u.includes('/credits')
    && !u.includes('flowWorkflows/');

  const origFetch = window.fetch;
  window.fetch = async function(input, init) {
    try {
      const url = typeof input === 'string' ? input : (input && input.url) || '';
      if (want(url)) {
        const method = (init && init.method) || (typeof input === 'object' && input && input.method) || 'GET';
        let headers = {};
        if (init && init.headers) {
          if (init.headers instanceof Headers) {
            init.headers.forEach((v, k) => { headers[k] = v; });
          } else if (Array.isArray(init.headers)) {
            for (const [k, v] of init.headers) headers[k] = v;
          } else { headers = Object.assign({}, init.headers); }
        }
        let body = (init && init.body) || null;
        if (body && typeof body !== 'string') { try { body = String(body); } catch(_) {} }
        window.__capRequests.push({ url, method, headers, body, t: Date.now(), via: 'fetch' });
      }
    } catch(e) {}
    return origFetch.apply(this, arguments);
  };

  const origOpen = XMLHttpRequest.prototype.open;
  const origSetH = XMLHttpRequest.prototype.setRequestHeader;
  const origSend = XMLHttpRequest.prototype.send;
  XMLHttpRequest.prototype.open = function(method, url) {
    this.__cap = { method, url, headers: {} };
    return origOpen.apply(this, arguments);
  };
  XMLHttpRequest.prototype.setRequestHeader = function(k, v) {
    if (this.__cap) this.__cap.headers[k] = v;
    return origSetH.apply(this, arguments);
  };
  XMLHttpRequest.prototype.send = function(body) {
    if (this.__cap && want(this.__cap.url)) {
      let b = body || null;
      if (b && typeof b !== 'string') { try { b = String(b); } catch(_) {} }
      window.__capRequests.push({
        url: this.__cap.url, method: this.__cap.method,
        headers: this.__cap.headers, body: b, t: Date.now(), via: 'xhr',
      });
    }
    return origSend.apply(this, arguments);
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

const drainJS = `JSON.stringify({reqs: (window.__capRequests || []).splice(0), rec: (window.__capRecaptcha || []).splice(0), url: location.href})`

// openModelDropdownAndPickVeo finds the model-config chip in the bottom bar
// (Radix dropdown trigger), opens it, dumps menu items, and clicks the first
// item whose text contains "Veo".
const openModelDropdownAndPickVeoJS = `
(async () => {
  const sleep = (ms) => new Promise(r => setTimeout(r, ms));

  // Find chip: bottom-bar Radix-style button with model-name keywords.
  let chip = null;
  for (const b of document.querySelectorAll('button[id^="radix-"]')) {
    const r = b.getBoundingClientRect();
    if (r.y < window.innerHeight * 0.6) continue;
    const t = (b.innerText || '').toLowerCase();
    if (/banana|veo|nano|imagen|wan|kling/i.test(t)) { chip = b; break; }
  }
  if (!chip) return JSON.stringify({err: 'chip_not_found', step: 'find-chip'});

  const before = (chip.innerText || '').replace(/\s+/g, ' ').trim();
  if (/veo/i.test(before)) {
    return JSON.stringify({status: 'already_veo', chipText: before});
  }

  chip.click();
  await sleep(800);

  const items = [];
  for (const el of document.querySelectorAll('[role="menuitem"], [role="menuitemradio"], [role="option"]')) {
    const r = el.getBoundingClientRect();
    if (r.width === 0 || r.height === 0) continue;
    items.push({
      text: (el.innerText || '').replace(/\s+/g, ' ').trim(),
      dataState: el.getAttribute('data-state'),
    });
  }

  // Find first menu item whose text contains "Veo".
  let veoItem = null;
  for (const el of document.querySelectorAll('[role="menuitem"], [role="menuitemradio"], [role="option"]')) {
    const t = (el.innerText || '');
    if (/veo/i.test(t)) { veoItem = el; break; }
  }
  if (!veoItem) {
    return JSON.stringify({err: 'no_veo_menuitem', step: 'pick-veo', menuItems: items, chipBefore: before});
  }

  veoItem.click();
  await sleep(700);

  // Read chip text again to confirm switch.
  let after = '';
  try { after = (chip.innerText || '').replace(/\s+/g, ' ').trim(); } catch(_) {}

  return JSON.stringify({status: 'switched', chipBefore: before, chipAfter: after, pickedText: (veoItem.innerText || '').slice(0, 100), menuItems: items});
})()
`

// fillPromptJS focuses the Slate editor and uses execCommand('insertText'),
// which is the canonical Slate-compatible insertion path.
const fillPromptJS = `
(async () => {
  const sleep = (ms) => new Promise(r => setTimeout(r, ms));
  const editor = document.querySelector('[data-slate-editor="true"][role="textbox"]');
  if (!editor) return JSON.stringify({err: 'editor_not_found'});

  editor.focus();
  await sleep(80);
  // Click into editor for good measure (Slate listens to mouse events).
  const r = editor.getBoundingClientRect();
  editor.dispatchEvent(new MouseEvent('mousedown', {bubbles: true, clientX: r.x + 10, clientY: r.y + 5}));
  editor.dispatchEvent(new MouseEvent('mouseup', {bubbles: true, clientX: r.x + 10, clientY: r.y + 5}));
  await sleep(80);

  // Clear any existing text.
  document.execCommand('selectAll', false);
  document.execCommand('delete', false);
  await sleep(40);
  document.execCommand('insertText', false, %q);
  await sleep(150);
  return JSON.stringify({status: 'filled', text: (editor.innerText || '').slice(0, 100)});
})()
`

// clickSubmitJS finds the rightmost button in the bottom bar that has
// "arrow_forward" + "Tạo"/"Create" text. Returns immediately after click.
const clickSubmitJS = `
(async () => {
  const sleep = (ms) => new Promise(r => setTimeout(r, ms));
  const candidates = [];
  for (const b of document.querySelectorAll('button, [role="button"]')) {
    const r = b.getBoundingClientRect();
    if (r.y < window.innerHeight * 0.6 || r.width === 0) continue;
    const t = (b.innerText || '').trim();
    // arrow_forward icon + "Tạo"/Create label, but NOT "add_2 Tạo" (that's
    // the add-media button at lower x).
    if (/arrow_forward/.test(t) && !/add_2/.test(t)) {
      candidates.push({b, t, x: r.x});
    }
  }
  if (candidates.length === 0) return JSON.stringify({err: 'no_submit_btn'});
  candidates.sort((a, b) => b.x - a.x);
  const btn = candidates[0].b;
  // Check disabled.
  const disabled = btn.disabled || btn.getAttribute('aria-disabled') === 'true';
  if (disabled) return JSON.stringify({err: 'submit_disabled', text: candidates[0].t.slice(0, 60)});
  btn.scrollIntoView({block: 'center'});
  await sleep(80);
  btn.click();
  return JSON.stringify({status: 'clicked', text: candidates[0].t.slice(0, 60)});
})()
`

type cdpTarget struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

func findProjectTab(port int) (*cdpTarget, error) {
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
	return nil, fmt.Errorf("không có tab labs.google")
}

var awaitPromise = func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
	return p.WithAwaitPromise(true)
}

// runEval is a small helper to evaluate a JS expression and unmarshal its
// stringified-JSON return value into a generic map for diagnostics.
func runEval(ctx context.Context, js string) (string, error) {
	var raw string
	err := chromedp.Run(ctx, chromedp.Evaluate(js, &raw, awaitPromise))
	return raw, err
}

func main() {
	port := flag.Int("port", 9222, "CDP port")
	prompt := flag.String("prompt", "a cat dancing in space", "prompt to insert")
	captureWindow := flag.Duration("capture", 30*time.Second, "how long to keep polling for captured requests after submit")
	flag.Parse()

	tab, err := findProjectTab(*port)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("attached to tab %s url=%s", tab.ID, tab.URL)

	// Use a single long-lived chromedp context. Don't defer browser-context
	// cancel — let the OS reclaim TCP at process exit. This avoids chromedp
	// closing the labs.google tab on cleanup.
	parent := context.Background()
	allocCtx, _ := chromedp.NewRemoteAllocator(parent, fmt.Sprintf("http://127.0.0.1:%d", *port))
	browserCtx, _ := chromedp.NewContext(allocCtx)
	tabCtx, _ := chromedp.NewContext(browserCtx, chromedp.WithTargetID(target.ID(tab.ID)))

	// 1. Install interceptor.
	if r, err := runEval(tabCtx, installInterceptorJS); err != nil {
		log.Fatalf("install: %v", err)
	} else {
		log.Printf("interceptor: %s", r)
	}

	// 2. Open model dropdown, pick Veo.
	if r, err := runEval(tabCtx, openModelDropdownAndPickVeoJS); err != nil {
		log.Fatalf("pick veo: %v", err)
	} else {
		log.Printf("pick-veo: %s", r)
	}
	time.Sleep(800 * time.Millisecond)

	// 3. Fill prompt.
	jsFill := fmt.Sprintf(fillPromptJS, *prompt)
	if r, err := runEval(tabCtx, jsFill); err != nil {
		log.Fatalf("fill: %v", err)
	} else {
		log.Printf("fill: %s", r)
	}
	time.Sleep(400 * time.Millisecond)

	// 4. Click submit.
	if r, err := runEval(tabCtx, clickSubmitJS); err != nil {
		log.Fatalf("submit: %v", err)
	} else {
		log.Printf("submit: %s", r)
	}

	// 5. Poll the interceptor for the captured request.
	deadline := time.Now().Add(*captureWindow)
	captured := false
	for time.Now().Before(deadline) {
		raw, err := runEval(tabCtx, drainJS)
		if err != nil {
			log.Printf("drain: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}
		var payload struct {
			Reqs []map[string]any `json:"reqs"`
			Rec  []map[string]any `json:"rec"`
			URL  string           `json:"url"`
		}
		_ = json.Unmarshal([]byte(raw), &payload)
		if len(payload.Reqs) > 0 || len(payload.Rec) > 0 {
			fmt.Println("=== capture tick ===")
			pretty, _ := json.MarshalIndent(payload, "", "  ")
			fmt.Println(string(pretty))
			for _, r := range payload.Reqs {
				if u, _ := r["url"].(string); strings.Contains(u, "batchAsync") || strings.Contains(u, ":generateVideo") || strings.Contains(u, "Generate") {
					fmt.Println("=== TARGET CAPTURED ===")
					captured = true
				}
			}
			if captured {
				break
			}
		}
		time.Sleep(1 * time.Second)
	}
	if !captured {
		log.Println("WARN: hết capture window, không thấy submit request thật")
	}
}
