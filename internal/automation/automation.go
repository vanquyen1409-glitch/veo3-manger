// Package automation drives a real Chrome instance over CDP to interact with
// Google Labs / Flow. Phase A scope: connect to a user-managed Chrome that's
// already running with --remote-debugging-port=9222, navigate to the Flow page,
// and extract the access token embedded in __NEXT_DATA__.
//
// Subsequent phases will add: prompt insertion (Slate.js), settings dropdown
// manipulation, submit + poll API calls, and signed-URL video download.
package automation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
)

// DefaultFlowURL is the Google Labs Flow tool URL. Override via Browser.Navigate
// when Google ships breaking URL changes.
const DefaultFlowURL = "https://labs.google/fx/tools/flow"

// Browser is a thin handle around a chromedp context tied to a remote Chrome.
// The owner of a Browser must call Close() when done; the Chrome process itself
// is not managed (it stays running, since the user owns it).
type Browser struct {
	ctx         context.Context
	cancelCtx   context.CancelFunc
	cancelAlloc context.CancelFunc
}

// Connect attaches to an already-running Chrome on http://127.0.0.1:<port>
// using its CDP endpoint. The port should match what the user passed via
// --remote-debugging-port=<port> when launching Chrome.
//
// The returned Browser must be closed via Close() to release the WS connection.
// Closing the Browser does NOT terminate Chrome.
func Connect(parent context.Context, port int) (*Browser, error) {
	if port <= 0 {
		port = 9222
	}
	url := fmt.Sprintf("http://127.0.0.1:%d", port)

	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(parent, url)
	browserCtx, cancelCtx := chromedp.NewContext(allocCtx)

	// Force the connection to materialize so that errors (Chrome not running,
	// port mismatch, etc.) surface here rather than in the first real action.
	verifyCtx, verifyCancel := context.WithTimeout(browserCtx, 5*time.Second)
	defer verifyCancel()
	if err := chromedp.Run(verifyCtx); err != nil {
		cancelCtx()
		cancelAlloc()
		return nil, fmt.Errorf("kết nối CDP thất bại: %w", err)
	}

	return &Browser{
		ctx:         browserCtx,
		cancelCtx:   cancelCtx,
		cancelAlloc: cancelAlloc,
	}, nil
}

// Close releases the CDP context. Safe to call multiple times.
func (b *Browser) Close() {
	if b == nil {
		return
	}
	if b.cancelCtx != nil {
		b.cancelCtx()
	}
	if b.cancelAlloc != nil {
		b.cancelAlloc()
	}
}

// NavigateAndExtractToken opens the given URL in a new browser tab, waits for
// the Next.js __NEXT_DATA__ script tag to be present, and extracts the
// authentication access token embedded in it.
//
// If the user is not signed in, the page typically redirects to a login page
// where __NEXT_DATA__ won't contain a token, and this returns an error so the
// caller can surface "please sign in" to the user.
//
// Phase A only: this just verifies the auth handshake works. Subsequent phases
// will use the returned token against the Veo3 generation API.
func (b *Browser) NavigateAndExtractToken(targetURL string) (string, error) {
	if b == nil || b.ctx == nil {
		return "", errors.New("browser chưa kết nối")
	}

	timeoutCtx, cancel := context.WithTimeout(b.ctx, 30*time.Second)
	defer cancel()

	var nextDataJSON string
	err := chromedp.Run(timeoutCtx,
		chromedp.Navigate(targetURL),
		chromedp.WaitReady("script#__NEXT_DATA__", chromedp.ByQuery),
		chromedp.Text("script#__NEXT_DATA__", &nextDataJSON, chromedp.ByQuery, chromedp.NodeVisible),
	)
	if err != nil {
		// Fallback: some Next.js setups don't expose script text via Text() —
		// pull it via raw JS evaluation instead.
		var fallback string
		if evalErr := chromedp.Run(timeoutCtx,
			chromedp.Navigate(targetURL),
			chromedp.WaitReady("script#__NEXT_DATA__", chromedp.ByQuery),
			chromedp.Evaluate(`document.getElementById('__NEXT_DATA__').textContent || ''`, &fallback),
		); evalErr != nil {
			return "", fmt.Errorf("đọc __NEXT_DATA__: %w (Text), %w (Eval)", err, evalErr)
		}
		nextDataJSON = fallback
	}

	if nextDataJSON == "" {
		return "", errors.New("__NEXT_DATA__ rỗng — có thể chưa đăng nhập")
	}

	token, err := ParseNextDataToken(nextDataJSON)
	if err != nil {
		return "", err
	}
	return token, nil
}
