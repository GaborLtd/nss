# Session Lifecycle

## 狀態

```text
CREATED → ATTACHED → DETACHED → ATTACHED
                         │
                         ├── CLOSE → CLOSED
                         └── TTL  → EXPIRED
```

### `CREATED`

session metadata 已建立，但尚未成功 attach 到 PTY。

### `ATTACHED`

有一個 client 持有 session attachment。預設禁止第二個 client 同時輸入。

### `DETACHED`

SSH transport 消失，但 PTY、shell 與 child process 仍然存活。`nss` client 進入 retry loop。

### `CLOSED`

使用者明確關閉 tab、執行 `nss close`，或 shell 正常結束。`nssd` 應清理 PTY、metadata 與 spool。

### `EXPIRED`

session 符合 cleanup policy，由 server 回收。

## Cleanup 原則

不能只用「最後一次 output 時間」判斷 session 是否可刪除。以下情況可能長時間沒有輸出但仍在工作：

- 編譯器或測試程序等待 input。
- `sleep` 或長時間計算。
- background job。
- server process 暫時沒有 log。

初版採保守策略。只有以下條件全部成立才可自動過期：

1. client 已 detached。
2. PTY 沒有 foreground child process。
3. shell 回到 prompt，或透過 shell integration 明確回報 idle。
4. input/output 已超過 idle TTL。
5. 沒有 active background job，或 background job 狀態已被可靠判定。

無法判定時不得自動刪除，交由管理指令處理。

建議初始 TTL 為 12～24 小時，完成實際使用測試後再調整；已連線的 session 不因 idle 自動清除。

## Attach policy

預設：

- 同一 session 只有一個 active attachment。
- 第二個 attach 拒絕並顯示目前 owner/attachment 狀態。
- 自動 reconnect 使用原本的 session secret。

進階管理：

```bash
nss list office-mini
nss attach office-mini <session-id>
nss takeover office-mini <session-id>
nss close office-mini <session-id>
```

`takeover` 必須明確操作。成功後舊 attachment 應收到可辨識的 `taken over` 狀態，而不是靜默地與新 client 互相搶輸入。

目前 MVP 已提供 `nssd list` 與 `nssd close --session-id`；跨裝置 takeover 與更完整的 admin control plane 仍屬下一階段。

## Crash 與重啟

- `nss` crash：remote session 保持 detached，直到 reconnect、明確 attach 或 cleanup。
- `nssd` crash：session 是否能恢復取決於 metadata 與 PTY 是否仍存在；初版可明確標記 session lost，不承諾跨 daemon crash 保存 PTY。
- server reboot：初版不承諾保存正在執行的 process；後續可透過 launchd、checkpoint 或更高階機制另行設計。
