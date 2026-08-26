# Server Operations

## 手動啟動

在 remote server 安裝 `nssd` 後，先以目前登入使用者啟動 daemon：

```bash
nssd serve
```

預設路徑：

```text
socket:    ~/.local/state/nss/nssd.sock
state dir: ~/.local/state/nss/
spool:     每個 session 4 MiB
```

可以覆寫：

```bash
nssd serve \
  --socket ~/.local/state/nss/nssd.sock \
  --state-dir ~/.local/state/nss \
  --max-spool-mb 16
```

`nssd` 不應該以 root 執行。socket 與 session state 只應由該 Unix user 存取。

## macOS launchd

可建立 `~/Library/LaunchAgents/com.gaborltd.nssd.plist`。將 `/Users/developer/.local/bin/nssd` 替換成實際路徑：

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.gaborltd.nssd</string>
  <key>ProgramArguments</key>
  <array>
    <string>/Users/developer/.local/bin/nssd</string>
    <string>serve</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>/tmp/nssd.stdout.log</string>
  <key>StandardErrorPath</key>
  <string>/tmp/nssd.stderr.log</string>
</dict>
</plist>
```

載入：

```bash
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.gaborltd.nssd.plist
launchctl kickstart -k gui/$(id -u)/com.gaborltd.nssd
```

## Linux systemd user service

建立 `~/.config/systemd/user/nssd.service`：

```ini
[Unit]
Description=Native Session Shell daemon

[Service]
ExecStart=%h/.local/bin/nssd serve
Restart=always
RestartSec=2

[Install]
WantedBy=default.target
```

啟用：

```bash
systemctl --user daemon-reload
systemctl --user enable --now nssd.service
```

## SSH smoke test

確認 server daemon 與 PATH：

```bash
ssh -T office-mini nssd --version
ssh -T office-mini nssd attach
```

`nss` 會在遠端 SSH command 執行前自動將 `~/.local/bin` 加入 `PATH`。因此若使用 installer 的預設位置，非互動 SSH shell 也能找到 `nssd`。但 `nssd` binary 必須安裝在遠端 machine，且 daemon 必須先啟動：

```bash
# 在 remote server 執行
nssd serve
```

若仍看到 `command not found: nssd`，請在 remote server 確認：

```bash
command -v nssd
~/.local/bin/nssd --version
```

若 `~/.local/bin/nssd` 存在但 `command -v nssd` 找不到，表示尚未完成遠端安裝或使用了不同的安裝路徑。

第二個指令會等待 `nss` protocol bytes，不適合直接在互動 terminal 手動輸入；它主要用於確認 binary 與 socket path。正式使用請執行：

```bash
nss office-mini
```

## Session 管理

管理指令透過同一個 Unix socket 執行，只有該 Unix user 可以使用：

```bash
nssd list
nssd close --session-id <session-id>

# 也可以透過一般 SSH 執行遠端管理指令
ssh -T office-mini nssd list
ssh -T office-mini nssd close --session-id <session-id>
```

`nssd close` 會結束對應 PTY 與 child process。跨裝置 takeover 目前尚未開放，避免兩個 terminal 同時寫入同一個 shell。

## 目前限制

- daemon restart 後目前不承諾恢復既有 PTY。
- idle cleanup 與跨裝置 takeover 尚未完成。
- spool 是有限 replay window，不保存無限 terminal history。
- `nss` 預設不保存斷線期間的 raw keyboard input。
- 強制使用 `kill -9` 無法執行任何 cleanup；若 terminal 因此仍處於 raw mode，可執行 `stty sane` 或重新開啟 terminal tab。正常 `SIGTERM`、`SIGINT` 與 `SIGHUP` 會先恢復 local terminal state。
