# Veo3 API Wire Shape (2026-05-05)

## Endpoint

POST to the Veo3 video-generation endpoint authenticated via session cookies + reCAPTCHA token.

## Request Envelope

```json
{
  "clientContext": {
    "recaptchaToken": "<token from grecaptcha.enterprise.execute>"
  },
  "requests": [
    {
      "textInput": {
        "prompt": "a cat walking on the moon"
      },
      "videoModelKey": "veo-3",
      "seed": 123456789,
      "metadata": {
        "sceneId": "scene-abc"
      }
    }
  ]
}
```

Key constraints:
- `seed` must be int32 (API rejects int64 with TYPE_INT32 error).
- `clientContext.recaptchaToken` required — omitting it causes 403.
- Top-level flat shape `{prompt, modelId, ...}` is **wrong** (old bug, now fixed).

Source types: `internal/api/types.go`. Client logic: `internal/api/client.go`.

## reCAPTCHA Flow

1. **Site key detection** — `Browser.DetectRecaptchaSiteKey()` in `internal/automation/recaptcha.go` scans the page DOM for the reCAPTCHA Enterprise script tag and extracts the key automatically.
2. **Token harvest** — `Browser.GetRecaptchaEnterpriseToken(siteKey, action)` executes via CDP:
   ```js
   grecaptcha.enterprise.execute(siteKey, { action: "submit" })
   ```
   Default action string: `"submit"` (may need adjustment — see Known Unknowns).
3. **Token placement** — sent in two places:
   - HTTP header: `X-Goog-Recaptcha-Token: <token>`
   - Request body: `clientContext.recaptchaToken`

## Verifying with cmd/uidrive

`cmd/uidrive/main.go` is a probe tool that intercepts `window.fetch` and hooks `grecaptcha.enterprise.execute` via Chrome DevTools Protocol against a running Chrome instance.

```bash
# Chrome must be running with remote debugging:
# chrome.exe --remote-debugging-port=9222

go run ./cmd/uidrive -prompt "test"
```

Captures the real outgoing request body — use this to diff against `internal/api/types.go` if the API rejects requests.

## Known Unknowns

- **reCAPTCHA action string**: `"submit"` is the current default; the real page may use a different action. Capture via `cmd/uidrive` to verify.
- **Exact field naming**: nested field names (e.g., `videoModelKey` vs `modelKey`) may shift. Always cross-check a live capture against `internal/api/types.go`.
- **Token TTL**: reCAPTCHA Enterprise tokens expire quickly; harvest immediately before the POST.

## Modified Files (2026-05-05)

| File | Change |
|---|---|
| `internal/api/types.go` | Wrapped envelope struct replacing flat shape |
| `internal/api/client.go` | Adds `X-Goog-Recaptcha-Token` header + token wiring |
| `internal/api/errors.go` | Handles TYPE_INT32 and reCAPTCHA error codes |
| `internal/automation/recaptcha.go` | New — `DetectRecaptchaSiteKey`, `GetRecaptchaEnterpriseToken` |
| `app_pipeline.go` | Wires reCAPTCHA harvest into generation pipeline |
