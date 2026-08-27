# Session Lifecycle

```text
CREATED → ATTACHED → DETACHED → ATTACHED
                         │
                         ├── CLOSE → CLOSED
                         └── TTL  → EXPIRED
```

- `CREATED`：metadata 已建立，但尚未成功 attach 到 PTY。
- `ATTACHED`：一個 client 持有 attachment，預設禁止第二個 client 同時輸入。
- `DETACHED`：SSH 消失，但 PTY、shell 與 child process 仍存活；`nss` 進入 retry loop。
- `CLOSED`：使用者明確關閉、執行 close，或 shell 正常結束；daemon 清理 PTY、metadata 與 spool。
- `EXPIRED`：符合 cleanup policy，由 server 回收。

## Cleanup 原則

不能只用最後一次 output 判斷是否可刪除，因為編譯、`sleep`、background job 或等待 input 都可能暫時沒有輸出。

初版只有在以下條件全部成立時才可自動過期：client 已 detached、PTY 沒有 foreground child、shell 回到 prompt 或明確回報 idle、input/output 超過 idle TTL，且沒有無法判定的 active background job。無法判定時交由管理指令處理。建議初始 TTL 為 12–24 小時；已連線 session 不因 idle 自動清除。

## Attach policy

同一 session 預設只有一個 active attachment。第二個 attach 應拒絕並顯示狀態；自動 reconnect 使用原本的 session secret。`takeover` 必須是明確操作，舊 attachment 應收到 `taken over` 狀態。

目前 MVP 提供：

```bash
nssd list
nssd close --session-id <session-id>
```

跨裝置 takeover 仍屬後續工作。

## Crash 與重啟

- `nss` crash：remote session 保持 detached，直到 reconnect、明確 attach 或 cleanup。
- `nssd` crash：初版不承諾跨 daemon crash 保存 PTY。
- server reboot：初版不承諾保存正在執行的 process。
