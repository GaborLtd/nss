# 架構

`nss` 將 SSH transport 與 remote terminal session 解耦：SSH connection 可以消失，但 `nssd` 管理的 remote PTY 應繼續存活。

```text
Terminal Tab A → nss session A → remote PTY + zsh
Terminal Tab B → nss session B → remote PTY + zsh
```

## 元件

### `nss`

- 取得 local terminal 的 stdin/stdout/stderr，並將 local TTY 設為 raw mode。
- 保存目前 tab 的 session identity。
- 啟動與監控系統 `ssh` process。
- transport 斷線後以 exponential backoff 重新連線。
- 重新執行 `ssh -T <host> nssd attach ...`，並轉送 resize/close。

### `nssd`

- 管理 session registry、remote PTY 與使用者 shell。
- attach 中斷後保留 PTY 與 child process。
- 將斷線期間的輸出寫入受 quota 限制的 disk spool。
- 執行 session ownership、attach 與 cleanup policy。

### SSH

OpenSSH 只負責 authentication 與 transport；`nssd` 自己管理 remote PTY。預期形式為：

```bash
ssh -T office-mini nssd attach --session-id <id> --secret <secret>
```

## Session identity 與 spool

每個 session 需要 human-readable ID、不可預測的 session secret、owner identity、時間戳、terminal dimensions 與 lifecycle state。session ID 不可單獨作為 authorization credential。

spool 必須有每 session 與 server 全域 quota、oldest-data drop 或 rotation、`output truncated` marker、0600 權限，並在 session close 後清理。這只保證有限的 replay window，不保證保存所有 terminal output。

## 非目標

- 不重新實作 SSH protocol。
- 不取代 terminal emulator 或管理 terminal tabs。
- 不做 tmux window/pane UI。
- 不自動重送斷線期間任意累積的 raw keyboard input。
- 不把 Cloudflare、VPN 或特定 tunnel 變成核心依賴。
