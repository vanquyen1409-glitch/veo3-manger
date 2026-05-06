// navproject is a tiny helper that connects to the running Chrome on
// :9222, finds the first labs.google /project/ link on the current Flow
// page, clicks it, and waits for the Slate prompt editor to be ready.
//
// Run BEFORE cmd/uidrive when Chrome is parked on the Flow homepage.
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

const findAndClickProjectJS = `
(() => {
  // First-pass: any link with /project/ in href.
  const links = Array.from(document.querySelectorAll('a[href*="/project/"]'));
  if (links.length > 0) {
    const a = links[0];
    a.scrollIntoView({block: 'center'});
    a.click();
    return JSON.stringify({clicked: 'link', href: a.href, text: (a.innerText||'').slice(0,80)});
  }
  // Fallback: project cards rendered as buttons / divs whose click handlers
  // navigate. Look for cards near the top with "Mở" or "Open" or just any
  // role=link inside a card-ish container.
  const cards = Array.from(document.querySelectorAll('[role="link"], [data-testid*="project"], div[class*="card"]'));
  const visible = cards.filter(c => {
    const r = c.getBoundingClientRect();
    return r.width > 50 && r.height > 50;
  });
  if (visible.length > 0) {
    visible[0].click();
    return JSON.stringify({clicked: 'card', text: (visible[0].innerText||'').slice(0,80)});
  }
  return JSON.stringify({err: 'no_project_link', anchorCount: document.querySelectorAll('a').length});
})()
`

type cdpTarget struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

func findFlowTab(port int) (*cdpTarget, error) {
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
	// Prefer /project/ first, else any labs.google.
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

func main() {
	port := flag.Int("port", 9222, "CDP port")
	flag.Parse()

	tab, err := findFlowTab(*port)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("attached: %s url=%s", tab.ID, tab.URL)

	if strings.Contains(tab.URL, "/project/") {
		log.Println("already on /project/ page, nothing to do")
		return
	}

	parent := context.Background()
	allocCtx, _ := chromedp.NewRemoteAllocator(parent, fmt.Sprintf("http://127.0.0.1:%d", *port))
	browserCtx, _ := chromedp.NewContext(allocCtx)
	tabCtx, _ := chromedp.NewContext(browserCtx, chromedp.WithTargetID(target.ID(tab.ID)))

	var raw string
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(findAndClickProjectJS, &raw)); err != nil {
		log.Fatalf("eval: %v", err)
	}
	log.Printf("click result: %s", raw)

	// Wait for navigation + Slate editor to appear (or timeout).
	waitCtx, cancel := context.WithTimeout(tabCtx, 15*time.Second)
	defer cancel()
	if err := chromedp.Run(waitCtx,
		chromedp.WaitVisible(`[data-slate-editor="true"][role="textbox"]`, chromedp.ByQuery),
	); err != nil {
		log.Printf("WARN: editor không xuất hiện trong 15s: %v", err)
		log.Println("→ user có thể cần click thủ công vào 1 project rồi rerun cmd/uidrive")
		return
	}

	// Re-fetch URL.
	var url string
	_ = chromedp.Run(tabCtx, chromedp.Evaluate(`location.href`, &url))
	log.Printf("OK editor sẵn sàng tại %s", url)
}
