# Veo3 Manager

Desktop application for managing Google Veo3 video-generation workflows.

## Tech Stack
- **Backend:** Go 1.21+ with Wails v2 (exposes typed methods to frontend)
- **Frontend:** React 18 + TypeScript (strict) + Vite
- **Styling:** Tailwind CSS
- **State:** Zustand
- **Notifications:** sonner
- **Date/Time:** date-fns

## Repository Layout
- `app.go` — `App` struct; **exported methods become Wails bindings callable from React**.
- `main.go` — Wails entrypoint, window options (title, size, background).
- `frontend/src/` — React app source.
- `frontend/wailsjs/` — **auto-generated**, do not edit by hand.
- `build/` — packaging assets and produced binaries.

## Common Commands
| Command | Purpose |
|---|---|
| `wails dev` | Dev mode with hot reload, browser devtools at http://localhost:34115 |
| `wails build` | Production binary in `build/bin/` |
| `wails doctor` | Diagnose missing system dependencies |
| `cd frontend && npm run dev` | Frontend-only dev server (no Wails bindings) |

## How Frontend ↔ Backend Communication Works
1. Add a method to the `App` struct in `app.go` (must be exported / PascalCase).
2. Run `wails dev` — Wails generates TS bindings into `frontend/wailsjs/go/main/App.ts`.
3. Import in React: `import { MyMethod } from "../wailsjs/go/main/App"`.
4. Call returns a Promise; awaitable from React.

## Conventions
- Keep Go business logic out of `app.go` once it grows; move to `internal/...` packages.
- Frontend uses TypeScript strict mode — no implicit `any`.
- Tailwind utilities only — no inline `style={}` unless dynamic value required.
- Zustand stores live in `frontend/src/stores/`.

## Don't
- Don't edit `frontend/wailsjs/` — it's regenerated.
- Don't commit `frontend/dist/` or `build/bin/`.
- Don't add competing UI/state frameworks (no Redux, no MUI, no Bootstrap).

## Initial Setup Notes (this environment)
- Go installed via `winget` at `C:\Program Files\Go\` (v1.26.2)
- Wails CLI at `%USERPROFILE%\go\bin\wails.exe` (v2.12.0)
- Node.js v24.15.0, npm 11.12.1
- WebView2 Runtime present (system-installed)

## Current State (2026-05-05)
- UI shell + mock backend complete; `wails build` produces ~11 MB exe.
- Bound methods (11): Settings.{Get,Save,DetectChromePath}, CDP.{GetStatus,Test}, Videos.{List,Create,Delete}, Misc.{GetSelectorConfigPath,OpenPathInOS,SelectFolder}.
- Events: `cdp:status`, `video:progress`, `videos:changed`.
- `/localfile/` HTTP handler in `main.go` serves files under configured `OutputDir` with path-traversal protection.
- Storage: JSON files at `%APPDATA%/veo3-manager/{settings,videos,selectors}.json`. Mutex-safe via `internal/store`.
- Tests: `go test ./...` covers store CRUD + handler security.
- Roadmap: see `docs/project-roadmap.md`.

## Real CDP Automation (2026-05-05)

API client now sends the correct wrapped envelope and reCAPTCHA token — three bugs fixed:

1. **Wire envelope**: request body is `{"clientContext": {...}, "requests": [{...}]}` — old flat `{prompt, modelId}` shape is gone (`internal/api/types.go`, `internal/api/client.go`).
2. **Seed type**: changed int64 → int32 to satisfy API TYPE_INT32 constraint.
3. **reCAPTCHA Enterprise**: `internal/automation/recaptcha.go` exposes `Browser.DetectRecaptchaSiteKey()` and `Browser.GetRecaptchaEnterpriseToken(siteKey, action)`; token sent as header `X-Goog-Recaptcha-Token` and body field `clientContext.recaptchaToken`. Pipeline wired in `app_pipeline.go`.

To capture a live request for diffing: `go run ./cmd/uidrive -prompt "test"` (Chrome must run with `--remote-debugging-port=9222`). See `docs/api-shape.md` for full wire shape + Known Unknowns.
