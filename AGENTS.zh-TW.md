# Native Session Shell（nss）專案規範（繁體中文）

本文件是 `AGENTS.md` 的繁體中文版本。英文版是 repository 的主要 contributor/agent 指南；若兩者不同，以英文版與目前使用者需求為準。

## 專案定位

- `nss` 是 laptop/mobile 端 CLI 與 reconnect orchestrator。
- `nssd` 是 server 端 session daemon，負責 PTY、shell process 與 lifecycle。
- SSH、VPN、LAN、Cloudflare Tunnel 等都是 transport layer，不得成為 session core 的依賴。
- 一個原生 terminal tab 對應一個獨立的遠端 PTY session。
- 不做 tmux 式多 window、多 pane、prefix hotkey 或內嵌 terminal UI。

## 開發規則

每個功能都需要實作、自動化測試、使用者或開發者文件與 CI 驗證。Protocol 必須可版本化並向後相容；網路斷線、process crash、重複 attach、session cleanup 與磁碟配額都需要測試。

本地至少執行：

```bash
gofmt -w .
go vet ./...
go test -race -cover ./...
go build ./cmd/nss ./cmd/nssd
```

## 安全與 release

不得在 log 或文件輸出 private key、session secret 或完整使用者輸入。Session secret 必須不可預測；spool 使用最小權限與 per-session/global quota；不得自動重送斷線期間 raw keyboard input；不得關閉 host key verification。

版本使用 `vMAJOR.MINOR.PATCH`。只有通過 CI 的 tag 才能建立 GitHub Release；release 必須包含 `nss`、`nssd`、checksum 與可重現版本資訊。Installer 必須先驗證 SHA-256，再安裝 GitHub Release assets。
