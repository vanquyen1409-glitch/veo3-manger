# Veo3 Manager — Roadmap

**Cập nhật**: 2026-05-05 (fixed video generation pipeline against real Google Labs Veo3 API; Phase B wire envelope + seed type + reCAPTCHA now live)

## Trạng thái hiện tại — UI Shell + Real CDP Pipeline (STRUCTURAL DONE)

App scaffold hoạt động end-to-end với mock generation. Build sạch, tests pass.

| Phần | Trạng thái | Ghi chú |
|---|---|---|
| Wails v2 skeleton | ✅ Done | Frameless tắt, window 1280×820, light theme |
| 3 trang UI: Tạo / Thư Viện / Cài Đặt | ✅ Done | Sidebar 240px + content area `bg-gray-50` |
| Zustand stores (4) | ✅ Done | nav, cdp, settings, videos |
| UI primitives (10) | ✅ Done | Button, Input, Textarea, Select, SegmentedControl, Card, Badge, EmptyState, ConfirmDialog, Field |
| sonner toast | ✅ Done | Top-right, richColors, closeButton |
| Wails bindings (11 method) | ✅ Done | Settings + CDP + Videos + Misc |
| File-backed JSON store | ✅ Done | `%APPDATA%/veo3-manager/{settings,videos,selectors}.json` |
| CDP probe `GET /json/version` | ✅ Done | Atomic state, không block UI |
| Auto-detect Chrome/Edge | ✅ Done | Scan Program Files + LOCALAPPDATA |
| `/localfile/` HTTP handler | ✅ Done | Path-traversal protected, percent-encoded support |
| Mock generation 7.8s | ✅ Done | Sẽ thay khi làm real CDP |
| Stale "generating" sweep on init | ✅ Done | Crash recovery |
| Auto-clear progress banner sau 4s | ✅ Done | Tránh stale UI |
| Go unit tests | ✅ Done | 48 test cases (10 store + 5 handler + 9 token + 14 API + 5 download + 5 reCAPTCHA), all pass |
| **Phase A.1: chromedp + CDP Connect** | ✅ Done | `internal/automation` package |
| **Phase A.2: Navigate labs.google + token extract** | ✅ Done | `__NEXT_DATA__` parser với 5 candidate paths |
| **Phase A.3: Wire vào CreateVideo flow** | ✅ Done | Stage 1+2 đã thật, stage 3+4 vẫn mock |

## Tiếp theo — Real CDP Automation

**Estimate ban đầu**: 1-2 tuần. **Đã ship Phase A.1-A.3 trong 1 session.**

### Phase B — API submit/poll ✅ Done (IN PROGRESS — wire shape fixed, awaiting one live test)
- [x] `internal/api/client.go` — `Client.Submit()`, `Client.Poll()`, `Client.WaitForCompletion()`
- [x] Bearer token auth via `TokenProvider` callback (cho phép re-extract khi 401)
- [x] `APIError` với HTTP status + truncated body (200→2000 chars) — actionable diagnostics
- [x] Auto-generate seeds (`crypto/rand` int32 — was int64, API rejected TYPE_INT32 overflow)
- [x] `MediaStatus` enum khớp Google: `MEDIA_GENERATION_STATUS_{PENDING,IN_PROGRESS,SUCCESSFUL,FAILED}`
- [x] Request wire envelope: `{"clientContext": {...}, "requests": [{...}]}` (was flat, API rejected)
- [x] Aspect-ratio + resolution mapped to enums: `VIDEO_ASPECT_RATIO_LANDSCAPE`, `RESOLUTION_720P`, etc.
- [x] reCAPTCHA Enterprise via CDP (`grecaptcha.enterprise.execute(siteKey, {action})`) harvested + sent in header + body field
- [x] 14 unit tests (httptest.Server fakes) — envelope shape, seed type, error truncation, poll, terminal success/failed/timeout, token errors
- [ ] **Verify real shape** — one live test against real API to confirm field names + reCAPTCHA action string
- [ ] 401 → re-extract token → retry — defer (hiện chỉ surface error)
- [ ] 429 backoff — defer

### Phase C — Download MP4 ✅ Done (structurally, BLOCKED on Phase B verification)
- [x] `internal/automation/download.go` — `Browser.DownloadVideo(redirectURL, dest, timeout)`
- [x] chromedp Network domain listener bắt response từ `storage.googleapis.com` / `lh3.googleusercontent.com`
- [x] HTTP GET signed URL → atomic write (`.part` → rename) vào configured `OutputDir`
- [x] `IsSignedVideoURL()` pure function + 7 test cases
- [x] `downloadFile()` test với httptest (success, HTTP error, atomic rename)
- [x] Wire vào `runGeneration` Stage 4 — replace mock với real download
- [x] Update `Video.FilePath` sau download → `videos:changed` event → UI hiển thị video player
- [ ] **Verify real flow** — blocked on Phase B verification; cần 1 lần test thực tế với Google redirect chain

### Phase A — Foundation ✅ Done
- [x] Thêm `github.com/chromedp/chromedp v0.15.1` (CGO-free, không cần MSVC)
- [x] Wrapper package `internal/automation/` (`automation.go` + `token.go`)
- [x] Connect tới Chrome đang chạy port 9222 qua `chromedp.NewRemoteAllocator`
- [x] Token extraction từ `__NEXT_DATA__` với 5 candidate JSON paths + fallback Eval
- [x] Wire vào `app.go runGeneration` — replace stage 1+2 của mock với thật
- [x] 9 unit tests cho `ParseNextDataToken` (standard path, alternate paths, first-hit-wins, empty-skip, malformed JSON, non-string defensiveness)
- [ ] Stealth payload — **defer** đến khi Google thực sự detect bot. chromedp đã giảm fingerprint đáng kể qua remote allocator (không launch Chrome mới).
- [ ] Launch Chrome mới nếu port 9222 không có — **defer**, hiện user phải tự launch Chrome với `--remote-debugging-port=9222`

### Phase B — Tạo video (3-5 ngày)
- [ ] Navigate đến labs.google
- [ ] Slate.js prompt input qua CDP `Input.insertText` (không phải keyboard event)
- [ ] Settings dropdown: aspect ratio (16:9/9:16), resolution, duration
- [ ] Filter nút Submit theo y > 680px
- [ ] Submit POST `/v1/video:batchAsyncGenerateVideoText` với Bearer token
- [ ] Poll status mỗi 10s, timeout 5 phút, terminal status `MEDIA_GENERATION_STATUS_SUCCESSFUL`

### Phase C — Tải video (2-3 ngày)
- [ ] Open redirect URL trong tab mới (cookies bound)
- [ ] Hijack request tới `storage.googleapis.com`, lấy signed URL
- [ ] HTTP GET signed URL + ghi file MP4 vào configured `OutputDir`
- [ ] Update `Video.FilePath` trong store
- [ ] `videos:changed` event trigger UI refresh

### Phase D — Hardening (2-3 ngày)
- [x] reCAPTCHA Enterprise token harvesting via CDP (new `internal/automation/recaptcha.go`) — STRUCTURAL DONE
- [ ] reCAPTCHA hard-challenge fallback → pause + UI prompt manual intervention — defer
- [ ] 401 → re-extract token, retry once
- [ ] 429 → backoff 5s/15s/45s
- [ ] Timeout / network errors → mark video failed với message rõ
- [ ] Selectors load từ `selectors.json` để dễ chỉnh khi Google đổi UI
- [ ] Logger middleware redact Bearer token

## Changelog — 2026-05-05

**Fixed video generation pipeline for real Google Labs Veo3 API:**
1. Request wire envelope: Changed from flat fields to `{"clientContext": {...}, "requests": [{...}]}` (API structure)
2. Seed type: Changed int64 → int32 (API rejected 63-bit values as TYPE_INT32 overflow)
3. reCAPTCHA Enterprise: Harvested via CDP `grecaptcha.enterprise.execute(siteKey, {action})`, sent in both `X-Goog-Recaptcha-Token` header + `clientContext.recaptchaToken` body field
4. Aspect-ratio + resolution: Mapped to wire enums (`VIDEO_ASPECT_RATIO_LANDSCAPE`, `RESOLUTION_720P`, etc.)
5. Error diagnostics: Error body truncation bumped 200→2000 chars for actionable error messages
- Files: `internal/api/{types,client,errors}.go`, `internal/automation/recaptcha.go`, `app_pipeline.go`, `internal/api/client_test.go`
- All `go test ./...` pass; `go build` clean.

## Backlog (sau real CDP)

- [ ] Batch queue (gửi nhiều prompt cùng lúc, pause/resume/stop) — đây là hướng plan ban đầu ở `veo3-manger-part1/plans/`
- [ ] Multi-output video carousel (1-4 video / prompt)
- [ ] Negative prompt + tags wired vào real API call
- [ ] History pagination cho > 1000 videos (hiện đang load all)
- [ ] Migration JSON → SQLite nếu dataset > 10k rows
- [ ] Code signing + NSIS installer
- [ ] Auto-update (Wails có support)

## Nợ kỹ thuật

| Item | Mức ưu tiên | Ghi chú |
|---|---|---|
| `wails generate module` warn `time.Time` not found | Thấp | Bindings vẫn work, TS dùng `any` cho time fields |
| `OpenPathInOS` Linux/macOS không quote path | Thấp | App Windows-only nhưng file Go cross-platform |
| Frontend hot-reload có thể giữ stale event listeners | Thấp | Module flag `listenersAttached` không reset trong dev hot-reload |
| Mỗi `ListVideos` đọc full slice từ memory map | Trung bình | OK đến ~10k rows, sau đó cần pagination |
| `selectors.json` chỉ là placeholder | Trung bình | Sẽ dùng khi làm real CDP |
