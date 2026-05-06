// probe dumps the bottom-bar UI structure on labs.google. With -click,
// it first dispatches a CDP mouse click on the model chip, waits, then
// dumps state — so we can see what actually changed.
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

const probeJS = `
(() => {
  const out = {url: location.href, chips: [], popovers: [], portals: [], slateExists: false};
  out.slateExists = !!document.querySelector('[data-slate-editor="true"][role="textbox"]');
  for (const b of document.querySelectorAll('button[id^="radix-"]')) {
    const r = b.getBoundingClientRect();
    if (r.width === 0) continue;
    out.chips.push({
      id: b.id,
      ariaExpanded: b.getAttribute('aria-expanded') || '',
      ariaHasPopup: b.getAttribute('aria-haspopup') || '',
      ariaControls: b.getAttribute('aria-controls') || '',
      dataState: b.getAttribute('data-state') || '',
      text: (b.innerText || '').replace(/\s+/g, ' ').trim().slice(0, 80),
      x: Math.round(r.x), y: Math.round(r.y), w: Math.round(r.width), h: Math.round(r.height),
    });
  }
  for (const el of document.querySelectorAll('[role="menu"], [role="listbox"], [data-radix-popper-content-wrapper], [data-radix-menu-content]')) {
    const r = el.getBoundingClientRect();
    out.popovers.push({
      role: el.getAttribute('role') || '',
      tag: el.tagName,
      attr: el.getAttributeNames().join(','),
      visible: r.width > 0 && r.height > 0,
      x: Math.round(r.x), y: Math.round(r.y), w: Math.round(r.width), h: Math.round(r.height),
      itemCount: el.querySelectorAll('[role="menuitem"], [role="menuitemradio"], [role="option"]').length,
      innerSnippet: (el.innerText || '').replace(/\s+/g, ' ').trim().slice(0, 200),
    });
  }
  for (const p of document.querySelectorAll('[id^="radix-"][role="menu"], [id^="radix-"][role="dialog"], div[id^="radix-"]')) {
    if (p.tagName === 'BUTTON') continue;
    const r = p.getBoundingClientRect();
    out.portals.push({
      id: p.id, role: p.getAttribute('role') || '', tag: p.tagName,
      x: Math.round(r.x), y: Math.round(r.y), w: Math.round(r.width), h: Math.round(r.height),
      visible: r.width > 0 && r.height > 0,
    });
  }
  return JSON.stringify(out);
})()
`

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
		if t.Type == "page" && strings.Contains(t.URL, "labs.google") {
			tt := t
			return &tt, nil
		}
	}
	return nil, fmt.Errorf("no labs.google tab")
}

func dump(ctx context.Context, label string) {
	var raw string
	if err := chromedp.Run(ctx, chromedp.Evaluate(probeJS, &raw)); err != nil {
		log.Fatalf("probe: %v", err)
	}
	var pretty any
	_ = json.Unmarshal([]byte(raw), &pretty)
	out, _ := json.MarshalIndent(pretty, "", "  ")
	fmt.Printf("\n========== %s ==========\n%s\n", label, out)
}

func main() {
	port := flag.Int("port", 9222, "CDP port")
	doClick := flag.Bool("click", false, "click the model chip then probe again")
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

	dump(tabCtx, "BEFORE")
	if *doClick {
		// Click chip via CDP mouse — Radix-compatible.
		// chip is at ~(867, 663) w=156 h=34 → center (945, 680)
		log.Println("dispatching MouseClickXY(945, 680)")
		if err := chromedp.Run(tabCtx, chromedp.MouseClickXY(945, 680)); err != nil {
			log.Fatalf("click: %v", err)
		}
		time.Sleep(1500 * time.Millisecond)
		dump(tabCtx, "AFTER CLICK")
	}
}
