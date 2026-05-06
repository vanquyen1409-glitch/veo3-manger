// token-test is a standalone diagnostic that connects to the running Chrome
// (CDP on port 9222), navigates to Flow, reads __NEXT_DATA__, dumps it to
// next_data.json, and then attempts to find the access token via the same
// candidate-path logic the app uses.
//
// Always writes next_data.json so we can inspect the JSON offline regardless
// of whether parsing succeeded.
//
// Usage: go run ./cmd/token-test
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/chromedp/chromedp"
	"veo3-manager/internal/automation"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println("[test] connecting to chrome on port 9222...")

	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(context.Background(), "http://127.0.0.1:9222")
	defer cancelAlloc()
	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	defer cancelCtx()

	runCtx, cancelRun := context.WithTimeout(ctx, 60*time.Second)
	defer cancelRun()

	log.Println("[test] navigating to about:blank, then Flow...")
	var nextData string
	err := chromedp.Run(runCtx,
		chromedp.Navigate("about:blank"),
		chromedp.Navigate(automation.DefaultFlowURL),
		chromedp.WaitReady("script#__NEXT_DATA__", chromedp.ByQuery),
		chromedp.Evaluate(`document.getElementById('__NEXT_DATA__')?.textContent || ''`, &nextData),
	)
	if err != nil {
		log.Printf("[test] CHROMEDP RUN FAILED: %v", err)
		os.Exit(1)
	}

	log.Printf("[test] __NEXT_DATA__ size=%d bytes", len(nextData))

	// Always dump the raw JSON for offline inspection.
	if err := os.WriteFile("next_data.json", []byte(nextData), 0o644); err != nil {
		log.Printf("[test] dump write failed: %v", err)
	} else {
		log.Println("[test] wrote next_data.json")
	}

	// Try the existing parser.
	token, err := automation.ParseNextDataToken(nextData)
	if err != nil {
		log.Printf("[test] PARSE FAILED: %v", err)
		log.Println("[test] inspect next_data.json to find the token's actual path")
		os.Exit(2)
	}
	log.Printf("[test] OK token len=%d", len(token))
}
