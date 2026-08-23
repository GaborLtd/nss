# Native Session Shell

Native Session Shell（`nss`）是一個給遠端開發使用的 native terminal session 工具。

它保留 tmux 最重要的能力：SSH 斷線時，遠端 shell 與正在執行的 process 仍然存活；但它不提供 tmux 的多 window、多 pane 或 prefix hotkey。

## 核心體驗

```text
Terminal Tab A
  └── nss session A
        └── remote PTY + zsh

Terminal Tab B
  └── nss session B
        └── remote PTY + zsh
```

一個原生 terminal tab 對應一個遠端 session。連線中斷時，`nss` 自動重試 SSH，連線恢復後重新 attach 到原本的 PTY。

## 設計邊界

- 使用系統 OpenSSH，不重新實作 SSH client。
- LAN、VPN、Cloudflare Tunnel、jump host 等都只是 SSH transport。
- `nss` 不攔截 tmux prefix，也不管理 terminal tabs。
- `nssd` 在遠端持有 PTY 與 shell process。
- 斷線期間的輸出使用有上限的 disk spool，不使用無界 memory。
- 不預設保存或重送斷線期間的 raw keyboard input。
- 預設不允許第二個 client 自動接管 session；takeover 必須明確操作。

## 目前狀態

目前 repository 已完成第一個可工作的 MVP vertical slice，並已建立：

- 專案工程規範與文件入口。
- Go module 與 `nss` / `nssd` executable。
- GitHub Actions CI。
- GitHub tag release 與 GoReleaser 設定。
- checksum 驗證的 installer scaffold。

目前已實作：

- `nssd serve`：管理 remote PTY 與 shell。
- `nssd attach`：透過 SSH stdin/stdout 轉送 attach protocol。
- `nss <host>`：使用系統 OpenSSH、自動 reconnect、PTY input/output 與 resize。
- 斷線期間的 bounded disk spool 與 reconnect replay。

仍待完成：session persistence across daemon restart、完整管理 CLI、idle cleanup 與 production deployment hardening。

## 開發

需要 Go toolchain。建議使用 `go.mod` 宣告的 Go 版本。

```bash
go test -race -cover ./...
go vet ./...
go build ./cmd/nss ./cmd/nssd
```

更完整的設計請閱讀：

- [架構](docs/architecture.md)
- [Session lifecycle](docs/lifecycle.md)
- [Protocol 初稿](docs/protocol.md)
- [Server deployment](docs/operations.md)
- [測試與 release process](docs/testing-and-release.md)

## 安裝方向

Release 後預期支援：

```bash
curl -fsSL https://raw.githubusercontent.com/gaborltd/nss/main/scripts/nss_install.sh | sh
```

installer 會從 `gaborltd/nss` 的 GitHub Release 下載對應平台的 archive，先驗證 SHA-256，再將 `nss` 與 `nssd` 安裝到 `~/.local/bin`。未來若有正式網域，可將同一支 script 暴露為 `https://YOUR-DOMAIN.example/nss_install.sh`。
