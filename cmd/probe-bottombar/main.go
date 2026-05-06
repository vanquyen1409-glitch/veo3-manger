// probe-bottombar dumps all clickable elements in the lower half of the
// labs.google project page so we can see what the submit button looks like.
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
  const out = {url: location.href, bottomBar: [], slateText: null, slateExists: false, modelChip: null};
  const editor = document.querySelector('[data-slate-editor="true"][role="textbox"]');
  if (editor) {
    out.slateExists = true;
    out.slateText = (editor.innerText || '').slice(0, 200);
  }
  for (const b of document.querySelectorAll('button, [role="button"]')) {
    const r = b.getBoundingClientRect();
    if (r.y < window.innerHeight * 0.5 || r.width === 0 || r.height === 0) continue;
    const t = (b.innerText || '').replace(/\s+/g,' ').trim();
    out.bottomBar.push({
      tag: b.tagName,
      text: t.slice(0, 80),
      x: Math.round(r.x), y: Math.round(r.y), w: Math.round(r.width), h: Math.round(r.height),
      disabled: b.disabled || b.getAttribute('aria-disabled') === 'true',
      ariaLabel: b.getAttribute('aria-label') || '',
      id: b.id || '',
      classes: (b.className || '').slice(0, 60),
    });
  }
  // Sort right-to-left, bottom-to-top
  out.bottomBar.sort((a,b) => b.x - a.x);
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
	if err := chromedp.Run(timeoutCtx, chromedp.Evaluate(probeJS, &raw)); err != nil {
		log.Fatal(err)
	}
	var pretty map[string]any
	_ = json.Unmarshal([]byte(raw), &pretty)
	out, _ := json.MarshalIndent(pretty, "", "  ")
	fmt.Println(string(out))
}
