# Veo3 Manager

Ứng dụng desktop Windows tự động tạo video AI từ Google Labs Veo3 thông qua điều khiển Chrome qua CDP.

> **Trạng thái hiện tại**: UI shell hoàn chỉnh + mock backend. Real CDP automation đang nằm trong roadmap — xem [docs/project-roadmap.md](./docs/project-roadmap.md).

## Tech Stack

- **Backend**: Go 1.23+, Wails v2 (WebView2)
- **Frontend**: React 19 + TypeScript (strict) + Vite + Tailwind CSS v4
- **State**: Zustand
- **Notifications**: sonner
- **Icons**: lucide-react
- **Storage**: JSON files dưới `%APPDATA%/veo3-manager/`

## Cài đặt

Yêu cầu: Go 1.23+, Node 20+, Wails CLI 2.12+, WebView2 Runtime (Windows 10+ thường có sẵn).

```bash
# Lần đầu
cd frontend
npm install

# Dev (hot-reload frontend, restart backend khi đổi Go)
wails dev

# Build production binary
wails build
# → build/bin/veo3-manager.exe (~11 MB)
```

## Sử dụng

1. Mở `veo3-manager.exe`.
2. Vào **Cài Đặt** → bấm **Tự động phát hiện** Chrome → **Lưu**.
3. Khởi động Chrome với CDP: `chrome.exe --remote-debugging-port=9222 --user-data-dir=C:\veo3-profile` (hoặc dùng shortcut tự tạo).
4. Trong app, bấm **Kiểm tra kết nối** ở Cài Đặt — phải thấy "Đã kết nối Chrome".
5. Vào **Tạo Video Mới**, nhập prompt, chọn aspect/resolution/duration, bấm **Tạo Video**.
6. Xem progress trực tiếp ở banner. Sau khi xong, video xuất hiện ở **Thư Viện Video**.

> Hiện tại Tạo Video chạy mock 7.8s và không sinh file thật. Real CDP automation đang là next phase.

## Cấu trúc thư mục

```
veo3-manager/
├── main.go                          # Wails entry + /localfile/ HTTP handler
├── app.go                           # Bound methods (Settings, CDP, Videos)
├── internal/
│   ├── types/types.go               # Data model
│   ├── store/store.go               # JSON file store (mutex-safe)
│   ├── store/store_test.go          # Unit tests
│   └── cdp/cdp.go                   # CDP probe + Chrome auto-detect
├── main_test.go                     # /localfile/ handler tests
├── docs/
│   └── project-roadmap.md           # Roadmap + technical debt
└── frontend/src/
    ├── App.tsx                      # Sidebar + content shell
    ├── components/
    │   ├── Sidebar.tsx
    │   └── ui/                      # 10 reusable primitives
    ├── pages/
    │   ├── CreatePage.tsx
    │   ├── LibraryPage.tsx
    │   └── SettingsPage.tsx
    ├── stores/                      # Zustand: nav / cdp / settings / videos
    └── lib/                         # format + statuses helpers
```

## Phát triển

```bash
# Chạy tests Go
go test ./...

# Type-check frontend (chạy như một phần của npm run build)
cd frontend && npm run build

# Generate Wails TS bindings sau khi sửa app.go
wails generate module
```

## Tài liệu

- [docs/project-roadmap.md](./docs/project-roadmap.md) — trạng thái + nợ kỹ thuật + phase tiếp theo
- [CLAUDE.md](./CLAUDE.md) — context cho AI agent
