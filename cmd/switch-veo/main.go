// switch-veo opens the model dropdown chip and clicks the first Veo item.
// Necessary because labs.google defaults new projects to Nano Banana 2
// (image gen), which doesn't trigger the video-submit reCAPTCHA action
// we want to capture.
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

const switchJS = `
(async () => {
  const sleep = (ms) => new Promise(r => setTimeout(r, ms));

  // 1. Find the model chip in the bottom bar.
  let chip = null;
  for (const b of document.querySelectorAll('button[id^="radix-"]')) {
    const r = b.getBoundingClientRect();
    if (r.y < window.innerHeight * 0.5) continue;
    const t = (b.innerText || '').toLowerCase();
    if (/banana|veo|nano|imagen|wan|kling/i.test(t)) { chip = b; break; }
  }
  if (!chip) return JSON.stringify({err: 'chip_not_found'});
  const before = (chip.innerText || '').replace(/\s+/g,' ').trim();
  if (/^.*veo/i.test(before)) return JSON.stringify({status: 'already_veo', chip: before});

  // 2. Open the dropdown.
  chip.click();
  await sleep(900);

  // 3. List menu items so caller can see what's there if Veo isn't found.
  const items = [];
  for (const el of document.querySelectorAll('[role="menuitem"], [role="menuitemradio"], [role="option"]')) {
    const r = el.getBoundingClientRect();
    if (r.width === 0 || r.height === 0) continue;
    items.push({text: (el.innerText||'').replace(/\s+/g,' ').trim().slice(0,80)});
  }

  // 4. Try to expand any "More models" / "Tất cả" submenu first.
  let expander = null;
  for (const el of document.querySelectorAll('button, [role="menuitem"], [role="button"]')) {
    const t = (el.innerText || '').toLowerCase();
    if (/more|tất cả|all models|xem thêm|see more/.test(t) && el !== chip) {
      const r = el.getBoundingClientRect();
      if (r.width > 0 && r.height > 0) { expander = el; break; }
    }
  }
  if (expander) {
    expander.click();
    await sleep(700);
  }

  // 5. Find and click the first Veo item.
  let veo = null;
  for (const el of document.querySelectorAll('[role="menuitem"], [role="menuitemradio"], [role="option"], button')) {
    const r = el.getBoundingClientRect();
    if (r.width === 0 || r.height === 0) continue;
    const t = (el.innerText || '');
    if (/\bveo\b/i.test(t) && el !== chip) { veo = el; break; }
  }
  if (!veo) return JSON.stringify({err: 'no_veo_item', chipBefore: before, menuItems: items, expanderClicked: !!expander});

  veo.click();
  await sleep(800);
  const after = (chip.innerText || '').replace(/\s+/g,' ').trim();
  return JSON.stringify({status: 'switched', before, after, picked: (veo.innerText||'').slice(0,80)});
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
		log.Fatal("không tìm thấy tab labs.google /project/")
	}

	parent := context.Background()
	allocCtx, _ := chromedp.NewRemoteAllocator(parent, fmt.Sprintf("http://127.0.0.1:%d", *port))
	browserCtx, _ := chromedp.NewContext(allocCtx)
	tabCtx, _ := chromedp.NewContext(browserCtx, chromedp.WithTargetID(target.ID(pageID)))

	timeoutCtx, cancel := context.WithTimeout(tabCtx, 15*time.Second)
	defer cancel()

	var raw string
	if err := chromedp.Run(timeoutCtx, chromedp.Evaluate(switchJS, &raw, awaitPromise)); err != nil {
		log.Fatalf("evaluate: %v", err)
	}
	var pretty map[string]any
	_ = json.Unmarshal([]byte(raw), &pretty)
	out, _ := json.MarshalIndent(pretty, "", "  ")
	fmt.Println(string(out))
}
