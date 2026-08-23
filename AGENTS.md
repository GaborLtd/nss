# Native Session Shell（nss）專案規範

## 專案定位

Native Session Shell（簡稱 `nss`）是一個建立在標準 OpenSSH 之上的遠端開發終端工具。

- `nss`：laptop/mobile 端的 CLI 與 reconnect orchestrator。
- `nssd`：server 端的 session daemon，負責 PTY、shell process 與 session lifecycle。
- SSH、VPN、LAN、Cloudflare Tunnel 或其他連線方式都屬於 transport layer，不得成為 `nss` 核心邏輯的依賴。
- 一個原生 terminal tab 對應一個獨立的遠端 PTY session。
- 不做 tmux 式的多 window、多 pane、prefix hotkey 或內嵌 terminal UI。

## 語言與文件

- 對使用者與專案文件一律使用繁體中文。
- 技術術語保留英文，必要時補上繁體中文說明。
- 程式碼註解使用繁體中文。
- 所有公開行為、CLI flags、設定檔、protocol 與 lifecycle 變更都必須同步更新文件。
- README 只放入門與穩定承諾；詳細設計放在 `docs/`。

## 功能開發規則

每一個功能都必須同時具備：

1. 實作。
2. 自動化測試。
3. 使用者或開發者文件。
4. CI 驗證。

沒有測試或文件的功能不得視為完成。若功能無法在單元測試中驗證，必須提供 integration test、可重現的 manual test procedure，或清楚記錄目前的限制。

優先順序如下：

1. 先定義行為與失敗情境。
2. 先寫測試，再寫實作；若不適用，至少在同一個變更中補齊測試。
3. 保持 protocol 可版本化、可向後相容。
4. 對網路斷線、process crash、重複 attach、session cleanup 與磁碟配額進行測試。

## 測試與品質門檻

本地開發至少執行：

```bash
gofmt -w .
go vet ./...
go test -race -cover ./...
go build ./cmd/nss ./cmd/nssd
```

CI 必須執行相同的主要檢查，並在 pull request 與主要分支 push 時驗證。新增 lint、static analysis 或 integration test 時，應同步加入 CI，而不是只依賴開發者本機。

測試不得依賴真實 production server、真實使用者憑證或未宣告的外部服務。需要 SSH、PTY 或檔案系統時，使用 test fixture、fake transport、temporary directory 或可控的 local test server。

## 安全規則

- 不在 log、錯誤訊息、測試 artifact 或文件中輸出 SSH private key、session secret 或完整使用者輸入。
- session secret 必須使用不可預測的隨機值，不得只依賴可猜測的 session ID。
- spool 檔案必須採取最小權限，並受單 session 與全域 disk quota 限制。
- 不自動送出斷線期間任意累積的 raw keyboard input。
- 不為了方便測試而關閉 host key verification 或認證檢查。
- GitHub Actions 使用最小必要的 `GITHUB_TOKEN` permissions；只有 release job 可以取得 `contents: write`。

## Release 規則

- 版本使用 Semantic Versioning，tag 格式為 `vMAJOR.MINOR.PATCH`。
- 只有通過 CI 的 tag 才能建立 GitHub Release。
- release 使用 GoReleaser 產生 macOS/Linux 的 `amd64` 與 `arm64` artifacts。
- Release 必須包含 `nss`、`nssd`、checksum 與可重現的版本資訊。
- installer 只能下載 GitHub Release assets，並且必須先驗證 SHA-256 checksum 再安裝。
- 破壞性 protocol 或設定變更必須在 release notes 與 migration 文件中說明。

## 變更檢查清單

提交或 pull request 前確認：

- [ ] 功能行為已寫入 `README.md` 或 `docs/`。
- [ ] 有單元測試或適當的 integration/manual test。
- [ ] `gofmt`、`go vet`、`go test -race` 與 build 通過。
- [ ] 沒有把 transport-specific 邏輯混入 session core。
- [ ] 沒有無界的 memory、disk spool、retry 或 goroutine 成長。
- [ ] 若改變 CLI、protocol、session lifecycle 或 release，已更新相關文件。
