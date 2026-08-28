# Native Session Shell

> Keep your native terminal. Keep your remote shell when the network disappears.

Native Session Shell (`nss`) is a small remote-development tool for interactive terminal sessions. It uses the OpenSSH client already installed on your machine, while `nssd` keeps the remote PTY and shell alive on the server.

## The one-line idea

If you normally use:

```bash
ssh office-mini
```

use this instead:

```bash
nss office-mini
```

`nss` keeps the familiar terminal experience and your existing SSH configuration. When Wi-Fi changes, a laptop sleeps, a phone switches networks, or a temporary outage drops SSH, `nss` retries the connection and reattaches to the same remote terminal session.

There is a one-time setup on the server: install `nssd` and register its user service. After that, everyday usage is just `nss <ssh-host>`.

## Why use nss?

The problem is simple: a normal SSH connection is also the lifetime of the remote interactive shell. When the connection dies, the terminal is gone—and a command that was running may be gone with it.

`nss` separates the network connection from the terminal session:

- One native terminal tab equals one remote session.
- Your terminal emulator still owns tabs, windows, copy/paste, shortcuts, and rendering.
- There is no tmux prefix key, pane navigation, or embedded terminal UI to learn.
- The remote PTY, shell, and running child process remain alive while the client is disconnected.
- Reconnection happens automatically when the network returns.
- Output produced during a short outage is replayed from bounded disk spool storage.
- SSH authentication, aliases, `ProxyJump`, VPNs, LAN connections, and tunnels continue to be handled by system OpenSSH.

The result is tmux-like session durability with a one-tab/one-session model and the native terminal workflow you already use.

## A session follows the tab

```text
Terminal Tab A                     Terminal Tab B
      │                                  │
      ▼                                  ▼
  nss session A                      nss session B
      │                                  │
      ▼                                  ▼
  remote PTY + shell                 remote PTY + shell
```

Opening another terminal tab creates another independent `nss` session. You do not attach to a shared tmux window and then search through multiple panes or windows. Closing a tab ends its session when the shell exits or the session is explicitly closed.

## A simple workflow

### 1. Install on both machines

Run the installer on the laptop and on the remote server. It installs both `nss` and `nssd`:

```bash
curl -fsSL https://raw.githubusercontent.com/gaborltd/nss/main/scripts/nss_install.sh | sh
```

The current release provides macOS and Linux binaries for `amd64` and `arm64`, and installs to `~/.local/bin` by default.

### 2. Start the server daemon once

On the remote server, register a user-level service:

```bash
nssd service install
nssd service status
```

macOS uses a LaunchAgent. Linux uses a systemd user service. If you prefer to run it in the foreground while testing:

```bash
nssd serve
```

Wait for the `nssd: ready` message, then leave the service running.

### 3. Connect like SSH

Use the same host alias you would pass to `ssh`:

```bash
nss office-mini
```

No new host database, tunnel configuration, or authentication system is required. `nss` invokes system OpenSSH and uses your normal `~/.ssh/config`.

### 4. Let the connection recover

If the laptop lid closes or the network changes, the tab remains open and `nss` enters reconnect mode. When the network returns, it attaches to the original remote PTY and resumes the same shell.

Do not type commands while disconnected. `nss` intentionally does not queue and replay arbitrary raw keyboard input, because replaying stale input could run an unintended command.

## How it compares

| Tool | What it is good at | The nss difference |
|---|---|---|
| [OpenSSH](https://www.openssh.com/) | A standard, scriptable remote connection | A dropped connection ends the interactive shell unless another session layer is used |
| [tmux](https://github.com/tmux/tmux) | Persistent sessions plus multiple windows and panes | Powerful, but adds a multiplexer UI, attach workflow, and prefix shortcuts |
| [Mosh](https://github.com/mobile-shell/mosh) | Resilient remote terminal transport and roaming between networks | Uses a separate transport and does not cover every SSH use case, such as port forwarding |
| [Eternal Terminal](https://github.com/MisterTea/EternalTerminal) | A reconnectable remote shell | Requires its own ET server/protocol deployment and its own client workflow |
| nss | A durable interactive session with one native tab per session | Keeps standard OpenSSH as the transport and leaves terminal UI to your terminal emulator |

The goal is not to replace these projects. `nss` chooses a narrower workflow: one tab, one remote PTY, automatic reconnect, and no multiplexer controls to memorize.

## Commands

```bash
# Open or reconnect to one remote terminal session.
nss <ssh-host>

# Update both nss and nssd on this machine.
nss update
nssd update

# Manage the remote daemon as a user service.
nssd service install
nssd service status
nssd service restart
nssd service uninstall

# Inspect or explicitly close remote sessions.
nssd list
nssd close --session-id <session-id>
```

`nss update` and `nssd update` do the same thing: both update `nss` and `nssd`, verify the release checksum, and atomically replace the local binaries. Updating does not restart a running daemon; restart its service afterward.

## What survives a disconnect?

While the SSH transport is down, `nssd` keeps the remote PTY, shell, and child processes alive. A bounded disk spool stores a finite replay window for output generated during the outage. It is not an unlimited terminal history and does not guarantee that every byte will be retained.

The current MVP does not yet promise PTY recovery across a daemon restart or server reboot. Cross-device takeover, automatic idle cleanup, and a Windows service backend are also future work.

## Design boundaries

- `nss` uses system OpenSSH; it does not implement a new SSH client.
- SSH, LAN, VPN, Cloudflare Tunnel, and jump hosts are transport choices, not `nss` core dependencies.
- `nss` does not manage terminal tabs or provide tmux-style windows and panes.
- The remote session secret is separate from the human-readable session ID.
- Spool storage, retry delays, and process growth are bounded.
- `nss` is intended for interactive terminal sessions. Use `ssh` directly for unrelated SSH features or non-interactive workflows.

## Documentation

- [Architecture](docs/architecture.md)
- [Session lifecycle](docs/lifecycle.md)
- [Attach protocol](docs/protocol.md)
- [Server operations](docs/operations.md)
- [Testing and release process](docs/testing-and-release.md)
- [Traditional Chinese README](README.zh-TW.md)
- [Traditional Chinese documentation](docs/zh-TW/README.md)

## Development

Go is required. Use the version declared by `go.mod`.

```bash
gofmt -w .
go vet ./...
go test -race -cover ./...
go build ./cmd/nss ./cmd/nssd
```
