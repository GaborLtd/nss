# Native Session Shell

> 保留 native terminal 體驗；即使網路消失，遠端 shell 也繼續存在。

Native Session Shell（`nss`）是給遠端開發使用的 interactive terminal session 工具。它使用機器上既有的 OpenSSH client，由遠端的 `nssd` 持有 PTY 與 shell，讓網路 connection 與 terminal session 分離。

## 一行就能理解

如果平常使用：

```bash
ssh office-mini
```

改成：

```bash
nss office-mini
```

`nss` 會沿用原本的 SSH config、host alias、authentication、`ProxyJump`、VPN、LAN 與 tunnel。Wi-Fi 切換、筆電睡眠、手機切換網路或短暫斷線時，`nss` 會自動重試，網路恢復後接回原本的 remote terminal session。

只需要在 server 做一次設定：安裝 `nssd` 並註冊 user service。之後日常使用就是 `nss <ssh-host>`。

## 為什麼使用 nss？

普通 SSH 的 connection 同時也是 remote interactive shell 的生命週期。connection 中斷時，terminal 消失，正在執行的 command 也可能一起停止。

`nss` 將 network connection 與 terminal session 分離：

- 一個原生 terminal tab 對應一個 remote session。
- Terminal emulator 仍然負責 tabs、windows、copy/paste、shortcuts 與畫面呈現。
- 不需要學 tmux prefix、pane navigation 或 embedded terminal UI。
- 斷線期間 remote PTY、shell 與 child process 保持運作。
- 網路恢復後自動 reconnect。
- 短暫斷線期間產生的 output 會由有上限的 disk spool replay。
- 不重新實作 SSH，沿用既有的認證與 SSH 設定。

也就是 tmux 的 session durability，加上 one-tab/one-session 與原生 terminal workflow。

## Session 跟著 tab 走

```text
Terminal Tab A                     Terminal Tab B
      │                                  │
      ▼                                  ▼
  nss session A                      nss session B
      │                                  │
      ▼                                  ▼
  remote PTY + shell                 remote PTY + shell
```

開啟另一個 terminal tab 就會建立另一個獨立的 `nss` session。不需要 attach 到共用的 tmux session 後，再從多個 window 或 pane 中尋找目標。

## 簡單流程

### 1. 在兩台機器安裝

在 laptop 與 remote server 都執行 installer；它會安裝 `nss` 與 `nssd`：

```bash
curl -fsSL https://raw.githubusercontent.com/gaborltd/nss/main/scripts/nss_install.sh | sh
```

目前 release 提供 macOS/Linux 的 `amd64` 與 `arm64` binary，預設安裝到 `~/.local/bin`。

### 2. 在 server 啟動一次 daemon

在 remote server 註冊 user-level service：

```bash
nssd service install
nssd service status
```

macOS 使用 LaunchAgent，Linux 使用 systemd user service。測試時也可以前景執行：

```bash
nssd serve
```

看到 `nssd: ready` 後，保持 service 運作即可。

### 3. 像 SSH 一樣連線

```bash
nss office-mini
```

不需要新的 host database、tunnel 設定或 authentication system。

### 4. 讓 connection 自動恢復

筆電蓋上或網路切換時，tab 會保持開啟並進入 reconnect mode。網路回來後會接回原本的 remote PTY 與 shell。

Reconnect 期間，`nss` 不會把 SSH diagnostic 直接混入 remote terminal 畫面，而是顯示短暫且會保存 cursor 位置的狀態列。下一次連線開始時狀態列會清除，不會永久覆蓋 remote prompt 或 terminal layout。

斷線期間不要輸入 command。為避免 stale input 造成誤執行，`nss` 不會保存並重送任意 raw keyboard input。

## 與其他工具的差異

- OpenSSH：標準且簡單，但 connection 中斷通常會結束 interactive shell。
- tmux：session 很強大，也提供 window/pane；但需要 multiplexer UI、attach 流程與 prefix shortcut。
- Mosh：專注於 resilient transport 與 network roaming，但使用另一套 transport，且不涵蓋所有 SSH 情境。
- Eternal Terminal：可 reconnect 的 remote shell，但需要自己的 server/protocol 與 client workflow。
- nss：使用標準 OpenSSH，讓一個 native tab 對應一個 session，並由 `nssd` 自動保存與恢復 remote PTY。

## 常用指令

```bash
nss <ssh-host>

# 更新這台機器上的 nss 與 nssd。
nss update
nssd update

# 管理 remote daemon service。
nssd service install
nssd service status
nssd service restart
nssd service uninstall

# 查看或明確關閉 remote session。
nssd list
nssd close --session-id <session-id>
```

`nss update` 與 `nssd update` 的效果相同：都會驗證 release checksum，並同時 atomic replace `nss` 與 `nssd`。更新不會自動重啟正在執行的 daemon。

## 目前限制

`nssd` 會在 SSH transport 斷線時保留 remote PTY、shell 與 child process。disk spool 只保存有限的 replay window，不保證保存所有 output，也不是無限 terminal history。

目前尚不保證 daemon restart 或 server reboot 後可以恢復既有 PTY；cross-device takeover、automatic idle cleanup 與 Windows service backend 也仍在後續規劃中。

和一般 `ssh <host>` 一樣，每個新的 remote session 都會從 authenticated user 的 home directory 開始。

## 文件

- [英文 README](README.md)
- [架構](docs/zh-TW/architecture.md)
- [Session lifecycle](docs/zh-TW/lifecycle.md)
- [Attach protocol](docs/zh-TW/protocol.md)
- [Server operations](docs/zh-TW/operations.md)
- [測試、CI 與 release](docs/zh-TW/testing-and-release.md)
