// probe-projects dumps any project-card-like elements visible on the
// labs.google Flow homepage to figure out the right selector.
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

const probeJS = `(() => {
  const out = {url: location.href, links: [], cards: [], buttons: [], heads: []};
  // h1 / h2 to know what page we're on
  for (const h of document.querySelectorAll('h1, h2, h3')) {
    const t = (h.innerText || '').replace(/\s+/g,' ').trim();
    if (t) out.heads.push(t.slice(0, 80));
  }
  // All anchors with href
  const anchors = Array.from(document.querySelectorAll('a[href]'));
  for (const a of anchors) {
    const href = a.getAttribute('href') || '';
    if (href.startsWith('javascript:') || href.startsWith('#')) continue;
    out.links.push({href, text: (a.innerText||'').replace(/\s+/g,' ').trim().slice(0,60)});
  }
  // Visible buttons that look like project cards (have aspect)
  for (const b of document.querySelectorAll('button, [role="button"], [role="link"]')) {
    const r = b.getBoundingClientRect();
    if (r.width < 100 || r.height < 60) continue;
    const t = (b.innerText || '').replace(/\s+/g,' ').trim();
    if (!t) continue;
    out.buttons.push({text: t.slice(0,80), x: Math.round(r.x), y: Math.round(r.y), w: Math.round(r.width), h: Math.round(r.height)});
  }
  // Generic large divs/cards that might be project tiles
  for (const c of document.querySelectorAll('div[class*="card"], [data-testid*="project"], article')) {
    const r = c.getBoundingClientRect();
    if (r.width < 100 || r.height < 100) continue;
    out.cards.push({text: (c.innerText||'').replace(/\s+/g,' ').trim().slice(0,60), tag: c.tagName, cls: c.className.slice(0,80)});
  }
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
		if t.Type == "page" && strings.Contains(t.URL, "labs.google") {
			pageID = t.ID
			break
		}
	}
	if pageID == "" {
		log.Fatal("no labs.google tab")
	}

	parent := context.Background()
	allocCtx, _ := chromedp.NewRemoteAllocator(parent, fmt.Sprintf("http://127.0.0.1:%d", *port))
	browserCtx, _ := chromedp.NewContext(allocCtx)
	tabCtx, _ := chromedp.NewContext(browserCtx, chromedp.WithTargetID(target.ID(pageID)))

	timeoutCtx, cancel := context.WithTimeout(tabCtx, 5*time.Second)
	defer cancel()

	var raw string
	if err := chromedp.Run(timeoutCtx, chromedp.Evaluate(probeJS, &raw)); err != nil {
		log.Fatal(err)
	}
	var pretty map[string]any
	_ = json.Unmarshal([]byte(raw), &pretty)
	out, _ := json.MarshalIndent(pretty, "", "  ")
	fmt.Println(string(out))
}
