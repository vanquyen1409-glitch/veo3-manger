// drain attaches to the labs.google tab on Chrome :9222 and polls
// window.__capRequests + window.__capRecaptcha (installed by cmd/uidrive)
// for a configurable window. Print the drained payload to stdout.
//
// Use after running cmd/uidrive once (which installs the interceptor)
// then manually triggering a real submit in the Chrome UI. The interceptor
// persists across page navigations within the same tab.
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

const drainJS = `JSON.stringify({
  reqs: (window.__capRequests || []).splice(0),
  rec: (window.__capRecaptcha || []).splice(0),
  url: location.href,
  installed: !!window.__capInstalled
})`

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

func main() {
	port := flag.Int("port", 9222, "CDP port")
	wait := flag.Duration("wait", 90*time.Second, "drain window")
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

	deadline := time.Now().Add(*wait)
	captured := false
	for time.Now().Before(deadline) {
		var raw string
		if err := chromedp.Run(tabCtx, chromedp.Evaluate(drainJS, &raw)); err != nil {
			log.Printf("eval: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}
		var p struct {
			Reqs      []map[string]any `json:"reqs"`
			Rec       []map[string]any `json:"rec"`
			URL       string           `json:"url"`
			Installed bool             `json:"installed"`
		}
		_ = json.Unmarshal([]byte(raw), &p)
		if !p.Installed {
			log.Println("WARN: interceptor chưa được install — chạy cmd/uidrive trước")
			return
		}
		if len(p.Reqs) > 0 || len(p.Rec) > 0 {
			fmt.Println("=== drain tick ===")
			pretty, _ := json.MarshalIndent(p, "", "  ")
			fmt.Println(string(pretty))
			for _, r := range p.Reqs {
				if u, _ := r["url"].(string); strings.Contains(u, "batchAsync") || strings.Contains(u, "Generate") {
					fmt.Println("=== TARGET CAPTURED — đã thấy submit request ===")
					captured = true
				}
			}
			if len(p.Rec) > 0 {
				fmt.Println("=== reCAPTCHA CAPTURED ===")
			}
			if captured && len(p.Rec) > 0 {
				return
			}
		}
		time.Sleep(1 * time.Second)
	}
	if !captured {
		log.Printf("hết %s, không thấy submit request — user cần switch sang Veo + nhấn Tạo trong Chrome", *wait)
	}
}
