package automation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// recaptchaEvalTimeout caps how long we wait for grecaptcha.enterprise.execute
// to resolve. The challenge itself is async (Google evaluates risk + may
// inject a visual challenge). 30s is generous but not infinite — a hung
// challenge means the user needs to interact in the browser window.
const recaptchaEvalTimeout = 30 * time.Second

// detectSiteKeyJS finds the reCAPTCHA Enterprise site key by scanning the
// page for the characteristic <script src="...recaptcha/enterprise.js?render=KEY">
// tag, which is how Google injects the runtime. Falls back to inspecting
// window.grecaptcha.enterprise's internal state if the script tag is gone.
//
// Returns "" if no key is found (common if the page hasn't loaded the
// reCAPTCHA bundle yet).
const detectSiteKeyJS = `
(() => {
  const scripts = Array.from(document.querySelectorAll('script[src]'));
  for (const s of scripts) {
    const m = s.src.match(/recaptcha\/enterprise\.js\?render=([A-Za-z0-9_\-]+)/);
    if (m) return m[1];
  }
  // Fallback: probe runtime-attached state. Different Google bundles store
  // the key on different paths; try a few. Use a WeakSet to break cycles —
  // Google's internal config objects are known to contain self-references
  // and a naive DFS would spin until the eval timeout fires.
  try {
    if (window.___grecaptcha_cfg && window.___grecaptcha_cfg.clients) {
      const seen = new WeakSet();
      for (const id in window.___grecaptcha_cfg.clients) {
        const c = window.___grecaptcha_cfg.clients[id];
        if (!c) continue;
        const stack = [c];
        while (stack.length) {
          const v = stack.pop();
          if (typeof v === 'string' && /^6L[a-zA-Z0-9_-]{30,}/.test(v)) return v;
          if (v && typeof v === 'object') {
            if (seen.has(v)) continue;
            seen.add(v);
            for (const k in v) {
              try { stack.push(v[k]); } catch (_) { /* getter throws */ }
            }
          }
        }
      }
    }
  } catch (e) { /* fall through */ }
  return '';
})()
`

// executeRecaptchaJS asks the page to fetch a fresh reCAPTCHA Enterprise
// token for a given (siteKey, action). The result is JSON-stringified so the
// chromedp Evaluate response is a plain string we can safely unmarshal.
//
// Note: grecaptcha.enterprise.execute returns a Promise; we return the
// stringified result so chromedp.Evaluate can capture it as a Go string
// rather than a JS Promise object.
const executeRecaptchaJS = `
(async () => {
  if (!(window.grecaptcha && window.grecaptcha.enterprise && window.grecaptcha.enterprise.execute)) {
    return JSON.stringify({err: 'grecaptcha.enterprise.execute không có sẵn'});
  }
  try {
    await new Promise((resolve) => {
      if (window.grecaptcha.enterprise.ready) {
        window.grecaptcha.enterprise.ready(resolve);
      } else {
        resolve();
      }
    });
    const token = await window.grecaptcha.enterprise.execute(%q, {action: %q});
    return JSON.stringify({token: token});
  } catch (e) {
    return JSON.stringify({err: String(e && e.message || e)});
  }
})()
`

// DetectRecaptchaSiteKey returns the reCAPTCHA Enterprise site key embedded
// in the current page, or "" if none found (in which case the caller should
// either skip reCAPTCHA or treat it as a hard failure).
func (b *Browser) DetectRecaptchaSiteKey(ctx context.Context) (string, error) {
	if b == nil || b.ctx == nil {
		return "", errors.New("browser chưa kết nối")
	}
	timeoutCtx, cancel := context.WithTimeout(b.ctx, 10*time.Second)
	defer cancel()

	var key string
	if err := chromedp.Run(timeoutCtx, chromedp.Evaluate(detectSiteKeyJS, &key)); err != nil {
		return "", fmt.Errorf("eval siteKey detect: %w", err)
	}
	key = strings.TrimSpace(key)
	if key == "" {
		log.Printf("[recaptcha] siteKey không tìm thấy trên trang")
	} else {
		log.Printf("[recaptcha] siteKey detected: %s", key)
	}
	return key, nil
}

// GetRecaptchaEnterpriseToken fetches a fresh reCAPTCHA Enterprise token
// for (siteKey, action). The token is single-use, lives ~30s–2min depending
// on Google's risk evaluation, and must be sent with the next API call.
//
// If grecaptcha is not present on the page, returns an error so the caller
// can decide whether to navigate first or skip reCAPTCHA entirely.
func (b *Browser) GetRecaptchaEnterpriseToken(ctx context.Context, siteKey, action string) (string, error) {
	if b == nil || b.ctx == nil {
		return "", errors.New("browser chưa kết nối")
	}
	if siteKey == "" {
		return "", errors.New("siteKey rỗng — không tìm thấy script reCAPTCHA trên trang")
	}
	if action == "" {
		action = "submit"
	}

	timeoutCtx, cancel := context.WithTimeout(b.ctx, recaptchaEvalTimeout)
	defer cancel()

	js := fmt.Sprintf(executeRecaptchaJS, siteKey, action)
	var raw string
	if err := chromedp.Run(timeoutCtx, chromedp.Evaluate(js, &raw, awaitPromise)); err != nil {
		return "", fmt.Errorf("evaluate grecaptcha: %w", err)
	}
	var out struct {
		Token string `json:"token"`
		Err   string `json:"err"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return "", fmt.Errorf("parse recaptcha result %q: %w", raw, err)
	}
	if out.Err != "" {
		return "", fmt.Errorf("grecaptcha.execute: %s", out.Err)
	}
	if out.Token == "" {
		return "", errors.New("recaptcha trả token rỗng")
	}
	log.Printf("[recaptcha] token len=%d action=%s", len(out.Token), action)
	return out.Token, nil
}

// awaitPromise is a chromedp.EvaluateOption that tells the JS runtime to
// await the returned Promise before resolving the Evaluate call. Without
// this, chromedp captures the Promise object instead of its resolved value.
func awaitPromise(p *runtime.EvaluateParams) *runtime.EvaluateParams {
	return p.WithAwaitPromise(true)
}

// installCaptureJS wraps grecaptcha.enterprise.execute so every call from the
// page itself records (siteKey, action) into window.__capRecaptcha. This lets
// us learn the EXACT action string the page uses (e.g. "GENERATE_VIDEO",
// "VEO3_SUBMIT") instead of guessing — Google's server-side risk evaluation
// requires the action issued at token-generation time to match what the
// backend expects, so a wrong action causes "reCAPTCHA evaluation failed"
// HTTP 403 even though the token is otherwise valid.
//
// The hook polls until grecaptcha is loaded (the bundle is async-loaded by
// labs.google) and is idempotent via window.__capRecaptchaInstalled. Designed
// to be installed via Page.addScriptToEvaluateOnNewDocument so it runs before
// any page script in every frame.
const installCaptureJS = `
(() => {
  if (window.__capRecaptchaInstalled) return;
  window.__capRecaptchaInstalled = true;
  if (!Array.isArray(window.__capRecaptcha)) window.__capRecaptcha = [];
  const wrap = () => {
    try {
      if (!(window.grecaptcha && window.grecaptcha.enterprise && window.grecaptcha.enterprise.execute)) {
        setTimeout(wrap, 100);
        return;
      }
      if (window.grecaptcha.enterprise.__capWrapped) return;
      window.grecaptcha.enterprise.__capWrapped = true;
      const orig = window.grecaptcha.enterprise.execute;
      window.grecaptcha.enterprise.execute = function(siteKey, options) {
        try {
          const action = options && typeof options === 'object' ? (options.action || '') : '';
          window.__capRecaptcha.push({ siteKey: String(siteKey || ''), action: String(action), t: Date.now() });
        } catch (_) {}
        return orig.apply(this, arguments);
      };
    } catch (_) {
      setTimeout(wrap, 200);
    }
  };
  wrap();
})()
`

// drainCaptureJS atomically removes and returns all captured entries via
// splice(0). Returns a JSON string so chromedp.Evaluate captures a Go string
// rather than a JS array (which it would unmarshal less predictably).
const drainCaptureJS = `JSON.stringify((window.__capRecaptcha || []).splice(0))`

// CapturedRecaptchaCall is one observation of grecaptcha.enterprise.execute()
// being called by the page itself. T is a Unix-millis timestamp from the page.
type CapturedRecaptchaCall struct {
	SiteKey string `json:"siteKey"`
	Action  string `json:"action"`
	T       int64  `json:"t"`
}

// InstallRecaptchaCapture registers the capture hook so that any future
// document loaded in this Browser auto-records calls to
// grecaptcha.enterprise.execute. Idempotent — calling repeatedly is safe.
//
// MUST be called BEFORE navigation for early-load grecaptcha calls to be
// captured (e.g. risk-evaluation preflights triggered on labs.google's
// initial page load). Also runs immediately on existing contexts via
// WithRunImmediately(true), so calling after navigation still works for
// future grecaptcha calls.
func (b *Browser) InstallRecaptchaCapture(ctx context.Context) error {
	if b == nil || b.ctx == nil {
		return errors.New("browser chưa kết nối")
	}
	timeoutCtx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
	defer cancel()

	err := chromedp.Run(timeoutCtx, chromedp.ActionFunc(func(c context.Context) error {
		_, err := page.AddScriptToEvaluateOnNewDocument(installCaptureJS).
			WithRunImmediately(true).
			Do(c)
		return err
	}))
	if err != nil {
		return fmt.Errorf("install recaptcha capture: %w", err)
	}
	log.Printf("[recaptcha] capture hook installed")
	return nil
}

// DrainCapturedRecaptcha returns and clears all entries the page has pushed
// since the last drain (or since hook install). Empty slice means the page
// has not yet called grecaptcha.enterprise.execute in this tab — typically
// because no submit-like UI action has been performed yet.
//
// As a side effect, the latest captured (siteKey, action) is cached on the
// Browser via LastRecaptcha() so subsequent submits in the same session can
// reuse it even if no fresh captures arrive (since splice(0) consumed them).
func (b *Browser) DrainCapturedRecaptcha(ctx context.Context) ([]CapturedRecaptchaCall, error) {
	if b == nil || b.ctx == nil {
		return nil, errors.New("browser chưa kết nối")
	}
	timeoutCtx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
	defer cancel()

	var raw string
	if err := chromedp.Run(timeoutCtx, chromedp.Evaluate(drainCaptureJS, &raw)); err != nil {
		return nil, fmt.Errorf("drain recaptcha capture: %w", err)
	}
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil, nil
	}
	var out []CapturedRecaptchaCall
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("parse capture %q: %w", truncateForLog(raw, 200), err)
	}
	if best := PickBestCapture(out, ""); best.Action != "" {
		b.mu.Lock()
		b.lastRecaptcha = best
		b.mu.Unlock()
	}
	return out, nil
}

// LastRecaptcha returns the most recent (siteKey, action) the Browser has
// ever observed via DrainCapturedRecaptcha across all submits in this
// session. Zero value if nothing observed yet. Intended as a fallback when
// a fresh drain returns empty — without this, the second submit in a
// session regresses to the hardcoded "submit" fallback even though the
// first submit learned the right action.
func (b *Browser) LastRecaptcha() CapturedRecaptchaCall {
	if b == nil {
		return CapturedRecaptchaCall{}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.lastRecaptcha
}

// PickBestCapture returns the most relevant captured call: prefer the latest
// non-empty (siteKey, action) pair. Returns zero value if calls is empty or
// no entry has a usable action. Action match against preferredSiteKey (when
// non-empty) is preferred to avoid grabbing a key from an unrelated widget
// (e.g. login form), but if no match is found the latest entry wins.
func PickBestCapture(calls []CapturedRecaptchaCall, preferredSiteKey string) CapturedRecaptchaCall {
	var fallback CapturedRecaptchaCall
	for i := len(calls) - 1; i >= 0; i-- {
		c := calls[i]
		if c.Action == "" {
			continue
		}
		if preferredSiteKey != "" && c.SiteKey == preferredSiteKey {
			return c
		}
		if fallback.Action == "" {
			fallback = c
		}
	}
	return fallback
}

// truncateForLog clips s for use in a log line. Identical semantics to the
// truncate helper in the api package; duplicated here to avoid an import.
func truncateForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
