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

第二個指令會等待 `nss` protocol bytes，不適合直接在互動 terminal 手動輸入；它主要用於確認 binary 與 socket path。正式使用請執行：

```bash
nss office-mini
```

## 目前限制

- daemon restart 後目前不承諾恢復既有 PTY。
- idle cleanup、session list、takeover 與 admin close 尚未完成。
- spool 是有限 replay window，不保存無限 terminal history。
- `nss` 預設不保存斷線期間的 raw keyboard input。
