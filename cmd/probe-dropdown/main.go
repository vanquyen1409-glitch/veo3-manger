// probe-dropdown clicks the model chip and dumps all newly-rendered
// elements, looking for what the menu actually looks like.
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

const probeJS = `
(async () => {
  const sleep = (ms) => new Promise(r => setTimeout(r, ms));
  let chip = null;
  for (const b of document.querySelectorAll('button[id^="radix-"]')) {
    const r = b.getBoundingClientRect();
    if (r.y < window.innerHeight * 0.5) continue;
    const t = (b.innerText || '').toLowerCase();
    if (/banana|veo|nano|imagen|wan|kling/i.test(t)) { chip = b; break; }
  }
  if (!chip) return JSON.stringify({err: 'no_chip'});
  chip.click();
  await sleep(1500);

  // Dump anything newly-large-and-visible.
  const out = {chip: (chip.innerText || '').replace(/\s+/g,' ').trim()};
  const all = [];
  for (const el of document.querySelectorAll('*')) {
    const r = el.getBoundingClientRect();
    if (r.width < 100 || r.height < 30) continue;
    const t = (el.innerText || '').replace(/\s+/g,' ').trim();
    if (!t || t.length > 200) continue;
    if (/\bveo\b/i.test(t) || /banana/i.test(t) || /nano/i.test(t) || /imagen/i.test(t)) {
      all.push({
        tag: el.tagName,
        role: el.getAttribute('role') || '',
        text: t.slice(0, 100),
        x: Math.round(r.x), y: Math.round(r.y), w: Math.round(r.width), h: Math.round(r.height),
        cls: (el.className || '').toString().slice(0, 80),
      });
    }
  }
  out.matches = all;
  // Also dump anything with role=menu / role=listbox
  out.menus = [];
  for (const el of document.querySelectorAll('[role="menu"], [role="listbox"], [data-state="open"]')) {
    const r = el.getBoundingClientRect();
    out.menus.push({
      role: el.getAttribute('role') || '',
      tag: el.tagName,
      visible: r.width > 0 && r.height > 0,
      x: Math.round(r.x), y: Math.round(r.y), w: Math.round(r.width), h: Math.round(r.height),
      childCount: el.children.length,
      sample: (el.innerText || '').replace(/\s+/g,' ').slice(0, 200),
    });
  }
  return JSON.stringify(out);
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
		log.Fatal("no project tab")
	}

	parent := context.Background()
	allocCtx, _ := chromedp.NewRemoteAllocator(parent, fmt.Sprintf("http://127.0.0.1:%d", *port))
	browserCtx, _ := chromedp.NewContext(allocCtx)
	tabCtx, _ := chromedp.NewContext(browserCtx, chromedp.WithTargetID(target.ID(pageID)))

	timeoutCtx, cancel := context.WithTimeout(tabCtx, 15*time.Second)
	defer cancel()

	var raw string
	if err := chromedp.Run(timeoutCtx, chromedp.Evaluate(probeJS, &raw, awaitPromise)); err != nil {
		log.Fatalf("evaluate: %v", err)
	}
	var pretty map[string]any
	_ = json.Unmarshal([]byte(raw), &pretty)
	out, _ := json.MarshalIndent(pretty, "", "  ")
	fmt.Println(string(out))
}
