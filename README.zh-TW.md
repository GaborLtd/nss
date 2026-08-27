# Native Session Shell

Native Session Shell（`nss`）是給遠端開發使用、具備 native terminal 體驗的 session 工具。

它保留 tmux 最重要的能力：SSH 斷線時，遠端 shell 與正在執行的 process 仍然存活；但不提供 tmux 式的 window、pane 或 prefix hotkey。

一個原生 terminal tab 對應一個獨立的遠端 PTY session。連線中斷時，`nss` 自動重試 SSH，網路恢復後重新 attach 到原本的 PTY。

## 設計邊界

- 使用系統 OpenSSH，不重新實作 SSH client。
- LAN、VPN、Cloudflare Tunnel、ProxyJump 等都只是 SSH transport。
- `nss` 不攔截 tmux prefix，也不管理 terminal tab。
- `nssd` 在遠端持有 PTY 與 shell process。
- 斷線期間的輸出使用有上限的 disk spool，不使用無界 memory。
- 不預設保存或重送斷線期間的 raw keyboard input。
- 第二個 client 不會靜默接管 session；takeover 必須明確操作。

## 目前狀態

目前已實作 `nssd serve`、`nssd attach`、`nssd update`、`nssd service install|status|restart|uninstall`、`nss <host>`、`nss update`，以及 bounded disk spool 與 reconnect replay。

仍待完成：daemon restart 後的 session persistence、takeover、idle cleanup、Windows service backend 與 production deployment hardening。

## 安裝與更新

```bash
curl -fsSL https://raw.githubusercontent.com/gaborltd/nss/main/scripts/nss_install.sh | sh
```

installer 會從 GitHub Release 下載對應平台的 archive，驗證 SHA-256 後將 `nss` 與 `nssd` 安裝到 `~/.local/bin`。

安裝後可以直接更新兩個 binary：

```bash
nss update
# 或
nssd update
```

兩個指令都會同時更新 `nss` 與 `nssd`。可使用 `--version v0.2.1` 固定版本。

## 文件

- [架構](docs/zh-TW/architecture.md)
- [Session lifecycle](docs/zh-TW/lifecycle.md)
- [Attach protocol](docs/zh-TW/protocol.md)
- [Server operations](docs/zh-TW/operations.md)
- [測試、CI 與 release](docs/zh-TW/testing-and-release.md)
- [English README](README.md)
