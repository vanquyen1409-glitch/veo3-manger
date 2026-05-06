// click-new-project clicks the "Dự án mới" / "New Project" button on the
// Flow homepage and waits for navigation to a /project/<id> page.
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

func awaitPromise(p *runtime.EvaluateParams) *runtime.EvaluateParams {
	return p.WithAwaitPromise(true)
}

const clickJS = `
(async () => {
  const sleep = (ms) => new Promise(r => setTimeout(r, ms));
  let btn = null;
  for (const b of document.querySelectorAll('button, [role="button"]')) {
    const t = (b.innerText || '').toLowerCase();
    if (/dự án mới|new project|tạo dự án/.test(t)) {
      const r = b.getBoundingClientRect();
      if (r.width > 50 && r.height > 50) { btn = b; break; }
    }
  }
  if (!btn) return JSON.stringify({err: 'no_new_project_btn'});
  btn.scrollIntoView({block: 'center'});
  await sleep(120);
  btn.click();
  return JSON.stringify({status: 'clicked', text: (btn.innerText||'').slice(0,80)});
})()
`

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

	var raw string
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(clickJS, &raw, awaitPromise)); err != nil {
		log.Fatalf("click: %v", err)
	}
	fmt.Println("click:", raw)

	// Poll URL for /project/.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var url string
		_ = chromedp.Run(tabCtx, chromedp.Location(&url))
		if strings.Contains(url, "/project/") {
			fmt.Println("now in project:", url)
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	log.Println("WARN: did not navigate to /project/ within 15s")
}
