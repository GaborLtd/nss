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

每個新 session 都會從 authenticated Unix user 的 home directory 開始，行為與 `ssh <host>` 的預設 working directory 一致；daemon service 自己的 working directory 不會影響 session 初始目錄。

## Service 管理

不需手寫 plist 或 systemd unit：

```bash
nssd service install
nssd service status
nssd service restart
nssd service uninstall
```

請不要使用 `sudo` 執行這些指令。它們會為目前帳號註冊 user-level service；使用 `sudo` 會改變 launchd/systemd 的 user context，並可能在 user 的 home directory 留下由 root 擁有的 service file。

如果不小心在 macOS 使用了 `sudo`，請先修正產生的 file ownership，再以目標 user 重新安裝：

```bash
sudo chown "$(id -un):$(id -gn)" "$HOME/Library/LaunchAgents/com.gaborltd.nssd.plist"
sudo chmod 600 "$HOME/Library/LaunchAgents/com.gaborltd.nssd.plist"
nssd service install
```

macOS 使用 LaunchAgent：`~/Library/LaunchAgents/com.gaborltd.nssd.plist`，並標記為同時支援 `Background` 與 `Aqua` session；在 SSH/headless 情境優先使用 `user/<uid>` launchd domain，必要時 fallback 到 `gui/<uid>`，也能辨識兩種 domain 中既有的 service。Linux 使用 systemd user service：`~/.config/systemd/user/nssd.service`。Windows service backend 尚未支援。Linux 若要在 logout 後保持服務，可使用 `loginctl enable-linger "$USER"`。

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

如果目前 user 的 `~/.ssh/config` 存在，`nss` 會明確將它傳給 OpenSSH，因此像 `mdev3` 這類 host alias 會使用與 `/usr/bin/ssh mdev3` 相同的設定。

### SSH passphrase prompt

在 interactive connection 或 reconnect 期間，如果 OpenSSH 要求 key passphrase，`nss` 會在目前 terminal 顯示 prompt。輸入 passphrase 後按 Enter；輸入只會傳給 SSH authentication process，不會保存到 session spool 或 log。支援 Backspace 與 `Ctrl-C`。

非互動模式（`--no-tty`）無法顯示 prompt，請先將 key 載入 SSH agent：

```bash
ssh-add ~/.ssh/id_ed25519
nss --no-tty office-mini
```

### Shell 環境變數與 prompt

每個由 `nss` 建立的 shell 都會收到：

| 變數 | 值 | 說明 |
|---|---|---|
| `NSS_SESSION` | `1` | 可供 shell 設定使用的穩定 marker。 |
| `NSS_SESSION_ID` | session ID | 僅適合顯示與診斷，不是 session secret。 |

如果使用 zsh，請將以下內容加到 remote `~/.zshrc`，並放在 prompt theme 初始化之後：

```zsh
if [[ -n "${NSS_SESSION:-}" ]]; then
    PROMPT="[nss:${NSS_SESSION_ID[1,8]}] ${PROMPT}"
fi
```

如果使用 bash，請將以下內容加到 `~/.bashrc`：

```bash
if [[ -n "${NSS_SESSION:-}" ]]; then
    PS1="[nss:${NSS_SESSION_ID:0:8}] ${PS1}"
fi
```

一般 SSH shell 不會收到 `NSS_SESSION`，所以這個條件不會改變一般 SSH 的 prompt。如果 prompt framework 會重新設定 `PROMPT` 或 `PS1`，請將這段放在 framework 初始化之後。

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
