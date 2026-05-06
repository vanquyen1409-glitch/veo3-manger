// check-hook attaches to the labs.google tab and verifies whether our
// capture hook successfully wrapped grecaptcha.enterprise.execute. Prints:
//   - whether grecaptcha is loaded on the page
//   - whether our wrapper marker (__capWrapped) is set
//   - the current size of __capRecaptcha
//
// Usage: go run ./cmd/check-hook
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

const checkJS = `(() => JSON.stringify({
  grecaptchaPresent: !!(window.grecaptcha && window.grecaptcha.enterprise && window.grecaptcha.enterprise.execute),
  hookInstalled: !!window.__capRecaptchaInstalled,
  wrapped: !!(window.grecaptcha && window.grecaptcha.enterprise && window.grecaptcha.enterprise.__capWrapped),
  bufferLen: (window.__capRecaptcha || []).length,
  url: location.href,
}))()`

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

	timeoutCtx, cancel := context.WithTimeout(tabCtx, 5*time.Second)
	defer cancel()

	var raw string
	if err := chromedp.Run(timeoutCtx, chromedp.Evaluate(checkJS, &raw)); err != nil {
		log.Fatalf("evaluate: %v", err)
	}
	fmt.Println(raw)
}
