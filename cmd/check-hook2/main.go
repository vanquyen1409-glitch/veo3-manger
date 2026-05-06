// check-hook2 inspects what reCAPTCHA APIs the page exposes, so we can
// determine which entry point the page actually calls. Some pages use
// grecaptcha.enterprise.execute, others grecaptcha.execute, and Google
// reCAPTCHA Enterprise v3 also exposes ready callbacks via different paths.
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

const inspectJS = `(() => JSON.stringify({
  url: location.href,
  has: {
    grecaptcha: !!window.grecaptcha,
    grecaptchaExecute: !!(window.grecaptcha && window.grecaptcha.execute),
    grecaptchaReady: !!(window.grecaptcha && window.grecaptcha.ready),
    grecaptchaEnterprise: !!(window.grecaptcha && window.grecaptcha.enterprise),
    grecaptchaEnterpriseExecute: !!(window.grecaptcha && window.grecaptcha.enterprise && window.grecaptcha.enterprise.execute),
    grecaptchaEnterpriseReady: !!(window.grecaptcha && window.grecaptcha.enterprise && window.grecaptcha.enterprise.ready),
    grecaptchaCfg: !!window.___grecaptcha_cfg,
  },
  cfgClients: window.___grecaptcha_cfg && window.___grecaptcha_cfg.clients ? Object.keys(window.___grecaptcha_cfg.clients) : [],
  scriptTags: Array.from(document.querySelectorAll('script[src*="recaptcha"]')).map(s => s.src),
  cookies: document.cookie.length > 0 ? 'present' : 'none',
  // What the most-recent grecaptcha config looks like:
  cfgSummary: (() => {
    try {
      const cfg = window.___grecaptcha_cfg;
      if (!cfg) return null;
      const out = {fns: !!cfg.fns, count: cfg.count};
      if (cfg.clients) {
        out.clientIds = Object.keys(cfg.clients);
        // Probe first client for sitekey
        for (const id in cfg.clients) {
          const c = cfg.clients[id];
          const stack = [c];
          const seen = new WeakSet();
          while (stack.length) {
            const v = stack.pop();
            if (typeof v === 'string' && /^6L[a-zA-Z0-9_-]{30,}/.test(v)) { out.firstSiteKey = v; break; }
            if (v && typeof v === 'object') {
              if (seen.has(v)) continue;
              seen.add(v);
              for (const k in v) { try { stack.push(v[k]); } catch(_){} }
            }
          }
          if (out.firstSiteKey) break;
        }
      }
      return out;
    } catch(e) { return {err: String(e)}; }
  })(),
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
	if err := chromedp.Run(timeoutCtx, chromedp.Evaluate(inspectJS, &raw)); err != nil {
		log.Fatalf("evaluate: %v", err)
	}
	// Pretty-print
	var pretty map[string]any
	_ = json.Unmarshal([]byte(raw), &pretty)
	out, _ := json.MarshalIndent(pretty, "", "  ")
	fmt.Println(string(out))
}
