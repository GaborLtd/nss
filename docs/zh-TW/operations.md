# Server Operations

## 手動啟動

```bash
nssd serve
```

預設為 `~/.local/state/nss/nssd.sock`、`~/.local/state/nss/`，每個 session 4 MiB spool。可用 `--socket`、`--state-dir` 與 `--max-spool-mb` 覆寫。不要以 root 執行 `nssd`。

成功啟動後會顯示：

```text
nssd: ready; socket=/Users/developer/.local/state/nss/nssd.sock; state-dir=/Users/developer/.local/state/nss; max-spool=4 MiB
```

另一個 terminal 可執行 `nssd list`；若沒有 session，會看到 `SESSION_ID\tATTACHED` 標題。

## Service 管理

不需手寫 plist 或 systemd unit：

```bash
nssd service install
nssd service status
nssd service restart
nssd service uninstall
```

macOS 使用 LaunchAgent：`~/Library/LaunchAgents/com.gaborltd.nssd.plist`；Linux 使用 systemd user service：`~/.config/systemd/user/nssd.service`。Windows service backend 尚未支援。Linux 若要在 logout 後保持服務，可使用 `loginctl enable-linger "$USER"`。

## SSH smoke test

```bash
ssh -T office-mini nssd --version
nss office-mini
```

`nss` 會在遠端 command 執行前將 `~/.local/bin` 加入 PATH。若仍看到 `command not found: nssd`，請在 server 確認：

```bash
command -v nssd
~/.local/bin/nssd --version
```

正常 interactive session 中，SSH diagnostic 不會直接混入 remote PTY 畫面。transport 斷線時，`nss` 會顯示短暫且保存 cursor 位置的 reconnect 狀態列，下一次連線開始前清除，讓 remote prompt 與 terminal layout 保持可讀。

## Session 管理與更新

```bash
nssd list
nssd close --session-id <session-id>
ssh -T office-mini nssd list
```

`nssd close` 會結束對應 PTY 與 child process。`nss update` 與 `nssd update` 都會從 GitHub Release 下載、驗證 `checksums.txt`，再 atomic replace 同一台機器上的兩個 binary；不會自動重啟正在執行的 daemon。

## 目前限制

- daemon restart 後不保證恢復既有 PTY。
- idle cleanup、跨裝置 takeover 與 Windows service 尚未完成。
- spool 只提供有限 replay window。
- 不預設保存斷線期間的 raw keyboard input。
- `kill -9` 無法執行 cleanup；若 terminal 留在 raw mode，可執行 `stty sane` 或開新 tab。正常 `SIGTERM`、`SIGINT`、`SIGHUP` 會先恢復 local terminal state。
