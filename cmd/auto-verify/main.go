// auto-verify navigates the existing Chrome tab to labs.google/fx/tools/flow,
// waits for it to load, installs the capture hook, and drives a UI submit
// (mimicking cmd/uidrive but with better Veo-model selection logic). Drains
// the captured (siteKey, action) and prints it. One-shot verification of
// the reCAPTCHA action capture path.
//
// Costs 1 video credit (it triggers a real submit click).
//
// Usage: go run ./cmd/auto-verify -prompt "test" -capture 60s
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

const checkSignedInJS = `(() => JSON.stringify({
  url: location.href,
  hasNextData: !!document.getElementById('__NEXT_DATA__'),
  loginPrompts: document.querySelector('a[href*="accounts.google"]') ? 'visible' : 'absent',
}))()`

// switchToVeoJS opens the model dropdown and clicks any item containing "Veo".
// It first tries the bottom-bar Radix chip; falls back to scanning all
// open dropdowns once the user has opened them.
const switchToVeoJS = `
(async () => {
  const sleep = (ms) => new Promise(r => setTimeout(r, ms));
  let chip = null;
  for (const b of document.querySelectorAll('button[id^="radix-"]')) {
    const r = b.getBoundingClientRect();
    if (r.y < window.innerHeight * 0.5) continue;
    const t = (b.innerText || '').toLowerCase();
    if (/banana|veo|nano|imagen|wan|kling/i.test(t)) { chip = b; break; }
  }
  if (!chip) return JSON.stringify({err: 'chip_not_found'});
  const before = (chip.innerText || '').replace(/\s+/g,' ').trim();
  if (/veo/i.test(before)) return JSON.stringify({status: 'already_veo', chip: before});

  chip.click();
  await sleep(900);

  // Find Veo item in any open menu / portal.
  let veo = null;
  for (const el of document.querySelectorAll('[role="menuitem"], [role="menuitemradio"], [role="option"], button')) {
    const t = (el.innerText || '');
    const r = el.getBoundingClientRect();
    if (r.width === 0 || r.height === 0) continue;
    if (/veo/i.test(t) && el !== chip) { veo = el; break; }
  }
  if (!veo) return JSON.stringify({err: 'no_veo_item', chipBefore: before});

  veo.click();
  await sleep(700);
  const after = (chip.innerText || '').replace(/\s+/g,' ').trim();
  return JSON.stringify({status: 'switched', before, after, picked: (veo.innerText||'').slice(0,80)});
})()
`

// Slate-compatible insertion via execCommand('insertText').
const fillPromptJS = `
(async () => {
  const sleep = (ms) => new Promise(r => setTimeout(r, ms));
  const editor = document.querySelector('[data-slate-editor="true"][role="textbox"]');
  if (!editor) return JSON.stringify({err: 'no_editor'});
  editor.focus(); await sleep(80);
  const r = editor.getBoundingClientRect();
  editor.dispatchEvent(new MouseEvent('mousedown', {bubbles: true, clientX: r.x+10, clientY: r.y+5}));
  editor.dispatchEvent(new MouseEvent('mouseup', {bubbles: true, clientX: r.x+10, clientY: r.y+5}));
  await sleep(80);
  document.execCommand('selectAll', false);
  document.execCommand('delete', false);
  await sleep(40);
  document.execCommand('insertText', false, %q);
  await sleep(150);
  return JSON.stringify({status: 'filled', text: (editor.innerText||'').slice(0,80)});
})()
`

const clickSubmitJS = `
(async () => {
  const sleep = (ms) => new Promise(r => setTimeout(r, ms));
  const cand = [];
  for (const b of document.querySelectorAll('button, [role="button"]')) {
    const r = b.getBoundingClientRect();
    if (r.y < window.innerHeight * 0.5 || r.width === 0) continue;
    const t = (b.innerText || '').trim();
    if (/arrow_forward/.test(t) && !/add_2/.test(t)) cand.push({b, t, x: r.x});
  }
  if (cand.length === 0) return JSON.stringify({err: 'no_submit'});
  cand.sort((a,b) => b.x - a.x);
  const btn = cand[0].b;
  if (btn.disabled || btn.getAttribute('aria-disabled') === 'true') {
    return JSON.stringify({err: 'submit_disabled', text: cand[0].t.slice(0,60)});
  }
  btn.scrollIntoView({block: 'center'}); await sleep(80);
  btn.click();
  return JSON.stringify({status: 'clicked', text: cand[0].t.slice(0,60)});
})()
`

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
	prompt := flag.String("prompt", "a quiet beach at sunset, slow waves", "prompt to insert")
	capture := flag.Duration("capture", 90*time.Second, "post-submit capture window")
	skipNav := flag.Bool("skip-nav", false, "do not navigate; assume current tab is already on a /project/ page")
	flag.Parse()

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json", *port))
	if err != nil {
		log.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var all []cdpTarget
	_ = json.Unmarshal(body, &all)

	// Prefer a labs.google /project/ page if one's open; else any page.
	var pageID string
	for _, t := range all {
		if t.Type == "page" && strings.Contains(t.URL, "/project/") {
			pageID = t.ID
			break
		}
	}
	if pageID == "" {
		for _, t := range all {
			if t.Type == "page" && strings.Contains(t.URL, "labs.google") {
				pageID = t.ID
				break
			}
		}
	}
	if pageID == "" {
		for _, t := range all {
			if t.Type == "page" {
				pageID = t.ID
				break
			}
		}
	}
	if pageID == "" {
		log.Fatal("no Chrome page tab found")
	}
	log.Printf("attached to tab id=%s", pageID)

	parent := context.Background()
	allocCtx, _ := chromedp.NewRemoteAllocator(parent, fmt.Sprintf("http://127.0.0.1:%d", *port))
	browserCtx, _ := chromedp.NewContext(allocCtx)
	tabCtx, _ := chromedp.NewContext(browserCtx, chromedp.WithTargetID(target.ID(pageID)))

	// 1. Register the capture hook on every new doc (covers the upcoming nav).
	if err := chromedp.Run(tabCtx, chromedp.ActionFunc(func(c context.Context) error {
		_, err := page.AddScriptToEvaluateOnNewDocument(installCaptureJS).
			WithRunImmediately(true).
			Do(c)
		return err
	})); err != nil {
		log.Fatalf("addScript: %v", err)
	}
	log.Println("[1/6] capture hook registered for new docs")

	// 2. Navigate to Flow (skip if user/caller already in a project).
	if !*skipNav {
		if err := chromedp.Run(tabCtx, chromedp.Navigate("https://labs.google/fx/tools/flow")); err != nil {
			log.Fatalf("navigate: %v", err)
		}
		log.Println("[2/6] navigated to labs.google/fx/tools/flow")
	} else {
		log.Println("[2/6] skip-nav: keeping current page")
	}

	// 3. Wait briefly + check sign-in state.
	time.Sleep(2 * time.Second)
	var stateRaw string
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(checkSignedInJS, &stateRaw)); err != nil {
		log.Fatalf("check signin: %v", err)
	}
	fmt.Println("[3/6] page state:", stateRaw)
	if strings.Contains(stateRaw, `"loginPrompts":"visible"`) {
		log.Println("WARN: sign-in prompts visible — user may need to log in. Continuing anyway, but submit may fail.")
	}

	// Re-evaluate the install JS on current doc (in case the navigate happened
	// before the script could run on the new doc — defensive).
	_ = chromedp.Run(tabCtx, chromedp.Evaluate(installCaptureJS, nil))

	// 4. Try to switch to a Veo model.
	var switchRaw string
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(switchToVeoJS, &switchRaw, awaitPromise)); err != nil {
		log.Printf("switch-veo: %v (continuing)", err)
	} else {
		fmt.Println("[4/6] switch-veo:", switchRaw)
	}
	time.Sleep(800 * time.Millisecond)

	// 5. Fill prompt.
	var fillRaw string
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(fmt.Sprintf(fillPromptJS, *prompt), &fillRaw, awaitPromise)); err != nil {
		log.Fatalf("fill: %v", err)
	}
	fmt.Println("[5/6] fill:", fillRaw)
	time.Sleep(500 * time.Millisecond)

	// 6. Click submit.
	var submitRaw string
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(clickSubmitJS, &submitRaw, awaitPromise)); err != nil {
		log.Fatalf("submit: %v", err)
	}
	fmt.Println("[6/6] submit:", submitRaw)

	// Drain loop.
	deadline := time.Now().Add(*capture)
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
	log.Println("done")
}
