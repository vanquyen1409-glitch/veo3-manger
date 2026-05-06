// test-capture calls grecaptcha.enterprise.execute via our hooked path,
// then drains __capRecaptcha to confirm the wrapper recorded the call.
// This is a sanity check — if this works, the wrapper is functional and
// the only reason live captures are empty is the page never calls.
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

const triggerJS = `(async () => {
  try {
    const t = await window.grecaptcha.enterprise.execute('6LdsFiUsAAAAAIjVDZcuLhaHiDn5nnHVXVRQGeMV', {action: 'PROBE_TEST_ACTION'});
    return JSON.stringify({ok: true, tokenLen: (t || '').length});
  } catch (e) {
    return JSON.stringify({ok: false, err: String(e && e.message || e)});
  }
})()`

const drainJS = `JSON.stringify((window.__capRecaptcha || []).splice(0))`

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
		if t.Type == "page" && (strings.Contains(t.URL, "/project/") || strings.Contains(t.URL, "labs.google")) {
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

	timeoutCtx, cancel := context.WithTimeout(tabCtx, 30*time.Second)
	defer cancel()

	var trig string
	if err := chromedp.Run(timeoutCtx, chromedp.Evaluate(triggerJS, &trig, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	})); err != nil {
		log.Fatalf("trigger: %v", err)
	}
	fmt.Println("trigger:", trig)

	time.Sleep(500 * time.Millisecond)
	var drained string
	if err := chromedp.Run(timeoutCtx, chromedp.Evaluate(drainJS, &drained)); err != nil {
		log.Fatalf("drain: %v", err)
	}
	fmt.Println("drained:", drained)
}
