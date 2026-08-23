# Architecture

## 目標

`nss` 將 SSH transport 與 remote terminal session 解耦：SSH connection 可以消失，但 `nssd` 管理的 remote PTY 不應因此消失。

```text
┌────────────────────┐      system ssh       ┌──────────────────────────┐
│ Laptop              │ ──────────────────── │ Office Mac mini          │
│                    │                       │                          │
│ nss client          │  stdin/stdout bytes  │ nssd                     │
│ local terminal TTY  │ ◀──────────────────▶ │ session registry          │
└────────────────────┘                       │ PTY + zsh/bash            │
                                             └──────────────────────────┘
```

`nss` 不需要知道 SSH 是直接走 office LAN、VPN、ProxyJump 或其他 tunnel。只要系統的 `ssh` 能建立 connection，`nss` 就能使用同一個 attach protocol。

## 元件

### `nss`

- 取得 local terminal 的 stdin/stdout/stderr。
- 將 local TTY 設為 raw mode，傳送 terminal bytes。
- 保存本次 tab 的 session identity。
- 啟動與監控系統 `ssh` process。
- 在 transport 斷線後以 exponential backoff 重新連線。
- 重新執行 `ssh -T <host> nssd attach ...`。
- 轉送 terminal resize 與明確的 close 訊號。

### `nssd`

- 由遠端 launchd 或其他 service manager 保持運作。
- 管理 session registry。
- 建立 remote PTY 並啟動使用者指定的 shell。
- 消費 PTY output，避免 PTY buffer 滿載阻塞 child process。
- 在 attach 中斷後保留 PTY。
- 保存受 quota 限制的 disk spool。
- 執行 session owner、attach、takeover 與 cleanup policy。

### system OpenSSH

OpenSSH 只負責 authentication 與 transport。`nssd` 自己管理 remote PTY，因此不應要求 OpenSSH 再配置一層 pseudo-terminal。

預期 attach 形式：

```bash
ssh -T office-mini nssd attach --session-id <id> --secret <secret>
```

## Session identity

每個 session 需要：

- human-readable session ID：用於顯示與管理。
- high-entropy session secret：用於 reconnect authorization。
- owner identity：至少綁定 SSH authenticated user。
- created、last attached、last input、last output timestamps。
- terminal dimensions。
- lifecycle state。

session ID 不得單獨作為 authorization credential。

## Transport 狀態與 session 狀態

兩者必須分離：

```text
transport: connected / disconnected / retrying
session:   attached / detached / closing / expired
```

SSH 斷線只改變 transport 狀態與 session attachment，不應直接等同於 session close。

## Output spool

連線期間：

```text
PTY output → active attachment
```

斷線期間：

```text
PTY output → bounded disk spool
                 └── small in-memory tail / metadata
```

必須有：

- 每 session 最大 bytes。
- server 全域最大 bytes。
- rotation 或 oldest-data drop policy。
- `output truncated` replay marker。
- 0600 file mode。
- session close 後清理 spool。

不能把「保存所有 terminal output」當成保證；保證範圍是有限 replay window。

## 非目標

- 不實作新的 SSH protocol。
- 不取代 terminal emulator。
- 不管理 terminal tabs。
- 不做 tmux window/pane UI。
- 不自動將斷線期間任意 raw keyboard input 重送。
- 不以 Cloudflare、VPN 或某個 tunnel 作為核心依賴。
