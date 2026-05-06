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
	"log"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

// DefaultFlowURL is the Google Labs Flow tool URL. Override via Browser.Navigate
// when Google ships breaking URL changes.
const DefaultFlowURL = "https://labs.google/fx/tools/flow"

// Browser is a thin handle around a chromedp context tied to a remote Chrome.
// The owner of a Browser must call Close() when done; the Chrome process itself
// is not managed (it stays running, since the user owns it).
//
// lastRecaptcha caches the most recent (siteKey, action) the Browser has ever
// observed via the capture hook. Subsequent submits in the same session can
// reuse this when the live drain is empty (e.g. the page didn't make any new
// grecaptcha calls between submits) — without the cache, video 2 in a
// session would regress to the hardcoded "submit" fallback even though video
// 1 already learned the correct action. Mu protects concurrent access since
// captures arrive from chromedp goroutines.
type Browser struct {
	ctx         context.Context
	cancelCtx   context.CancelFunc
	cancelAlloc context.CancelFunc

	mu            sync.Mutex
	lastRecaptcha CapturedRecaptchaCall
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

// CurrentURL returns the URL of the currently focused tab. Used by the
// pipeline to extract the Flow project ID from /project/<id> for inclusion
// in the submit envelope's clientContext.projectId. Returns "" on error
// (errors are logged but non-fatal — projectId is optional in the wire shape).
func (b *Browser) CurrentURL(ctx context.Context) string {
	if b == nil || b.ctx == nil {
		return ""
	}
	timeoutCtx, cancel := context.WithTimeout(b.ctx, 3*time.Second)
	defer cancel()

	var url string
	if err := chromedp.Run(timeoutCtx, chromedp.Location(&url)); err != nil {
		log.Printf("[automation] đọc URL hiện tại thất bại: %v", err)
		return ""
	}
	return url
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
// Before navigation, the reCAPTCHA capture hook is installed via
// Page.addScriptToEvaluateOnNewDocument so the page's own
// grecaptcha.enterprise.execute calls (e.g. preflight risk evaluations on
// page load) are observed. The captured (siteKey, action) pairs are drained
// later in the submit flow to learn the real action string the page uses —
// hardcoding "submit" produced HTTP 403 reCAPTCHA evaluation failures.
//
// If the user is not signed in, the page typically redirects to a login page
// where __NEXT_DATA__ won't contain a token, and this returns an error so the
// caller can surface "please sign in" to the user.
func (b *Browser) NavigateAndExtractToken(targetURL string) (string, error) {
	if b == nil || b.ctx == nil {
		return "", errors.New("browser chưa kết nối")
	}

	// Best-effort: install capture hook BEFORE navigation. Don't fail the
	// whole flow if the hook can't be installed — we degrade to the legacy
	// fallback (hardcoded "submit" action) which is no worse than today.
	if err := b.InstallRecaptchaCapture(b.ctx); err != nil {
		log.Printf("[automation] cài capture hook thất bại (sẽ tiếp tục): %v", err)
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
