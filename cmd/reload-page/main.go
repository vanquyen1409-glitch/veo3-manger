// reload-page reloads the labs.google project tab to recover from a
// client-side error.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

type cdpTarget struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	URL  string `json:"url"`
}

func main() {
	resp, err := http.Get("http://127.0.0.1:9222/json")
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
	allocCtx, _ := chromedp.NewRemoteAllocator(parent, "http://127.0.0.1:9222")
	browserCtx, _ := chromedp.NewContext(allocCtx)
	tabCtx, _ := chromedp.NewContext(browserCtx, chromedp.WithTargetID(target.ID(pageID)))

	if err := chromedp.Run(tabCtx, chromedp.ActionFunc(func(c context.Context) error {
		return page.Reload().Do(c)
	})); err != nil {
		log.Fatalf("reload: %v", err)
	}
	fmt.Println("reloaded")
}
