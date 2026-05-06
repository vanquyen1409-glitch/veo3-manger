# reCAPTCHA Action Hijack Fix (HTTP 403 PERMISSION_DENIED)

**Date:** 2026-05-05  **Owner:** vanquyen1409@gmail.com

## Recommended Approach: **A (Capture-then-replay, passive) with C-style cache**

Reasoning: minimal architectural change, keeps direct API submission, no UI driving. Install the JS hook on every browser connection; record `(siteKey, action)` whenever the page calls `grecaptcha.enterprise.execute` (which it does on its own probe/preflight `ready()` calls and any prior in-tab submit). Cache the first observation for the process lifetime. If no observation by submit time, fall back to today's hardcoded `"submit"` (current behavior — no regression). Approach D (self-trigger) and B (UI-drive) are deferred; if A still 403s in production we escalate to D as a follow-up.

## File-by-file changes

1. **`internal/automation/recaptcha.go`** — add new artifact + APIs:
   - Add `installCaptureJS` const: minimal subset of `cmd/uidrive/main.go::installInterceptorJS` lines 91-102 — only the `grecaptcha.enterprise.execute` wrapper that pushes `{siteKey, action, t}` to `window.__capRecaptcha`. Idempotent via `window.__capRecaptchaInstalled`.
   - `func (b *Browser) InstallRecaptchaCapture(ctx) error` — runs `installCaptureJS` once per Browser. Logs `[recaptcha] capture hook installed`.
   - `func (b *Browser) DrainCapturedRecaptcha(ctx) ([]CapturedCall, error)` — `JSON.stringify((window.__capRecaptcha||[]).splice(0))`. Returns `[]CapturedCall{SiteKey, Action, T}`.
   - Type `CapturedCall struct { SiteKey, Action string; T int64 }`.

2. **`internal/automation/browser.go`** (or wherever `NavigateAndExtractToken` lives) — call `InstallRecaptchaCapture` immediately after navigation succeeds. Non-fatal on error (log only).

3. **`app_pipeline.go::submitAndWait`** (lines 88-140):
   - After `DetectRecaptchaSiteKey`, call `browser.DrainCapturedRecaptcha(a.ctx)`.
   - If captured non-empty: pick the most recent entry; override `siteKey` if non-empty in capture; pass captured `action` via `api.WithRecaptchaAction(captured.Action)`.
   - Log `[pipeline] using siteKey=%s action=%s source=(captured|fallback)`.

4. **`internal/api/types.go`** — add `ProjectID` extraction helper (low-confidence secondary). Add `func ParseProjectIDFromURL(u string) string` matching `/project/([^/?#]+)`. Wire in `submitAndWait`: read `browser.CurrentURL(ctx)` (add if missing) → set `req.ProjectID`. Optional for this fix.

5. **`internal/api/client.go`** — no signature changes. Already supports `WithRecaptchaAction`. Add a one-line log when action != `DefaultRecaptchaAction`: `log.Printf("[api] recaptcha action overridden to %q", c.recaptchaAction)`.

## Testing strategy

**Unit (new):**
- `recaptcha_test.go::TestParseCapturedRecaptcha` — feed sample JSON `[{"siteKey":"6L...","action":"FLOW_SUBMIT","t":123}]`, assert decode.
- `client_test.go::TestSubmit_UsesOverrideAction` — construct client with `WithRecaptchaAction("FLOW_SUBMIT")`, mock provider asserts it receives `"FLOW_SUBMIT"`, response body envelope contains `recaptchaAction":"FLOW_SUBMIT"`.
- `types_test.go::TestParseProjectIDFromURL` — URL fixture coverage.

**Existing tests:** unchanged; `WithRecaptchaAction` empty-string guard already preserves default.

**Live verification:**
- Run app, generate one video, inspect logs for `[recaptcha] capture hook installed` → `[pipeline] using siteKey=... action=... source=captured`.
- If 403 persists, inspect drained `__capRecaptcha` JSON in logs to compare against our submitted action.

## Risk assessment

- **Low**: hook is idempotent and only wraps `execute`; no impact on legitimate page calls. Fallback path preserves current behavior on hook failure.
- **Medium**: page may genuinely never call grecaptcha before our submit (fresh tab, no prior submit) → no capture → fallback `"submit"` → still 403. Mitigation: log clearly + plan B/D follow-up.
- **Low**: ProjectID extraction wrong → 400 not 403; isolated change behind log.

## Effort

- Code: ~120 LOC across 4 files. **2-3 hours**.
- Tests: 3 new cases. **45 min**.
- Manual verification: **30 min**.

**Total: half a day.**

## Open questions

- Does labs.google call `grecaptcha.enterprise.execute` at page-load time, or only on submit? If only on submit, Approach A degrades to Approach D and we need self-trigger.
- Is `recaptchaAction` validated server-side as exact-match or prefix-match? Affects whether one capture is enough or we need per-flow re-capture.
