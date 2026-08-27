# Native Session Shell

Native Session Shell (`nss`) is a native-feeling terminal session tool for remote development.

It keeps the most important tmux capability: the remote shell and running processes survive an SSH disconnect. It does not provide tmux-style windows, panes, or prefix hotkeys.

## Core experience

```text
Terminal Tab A
  └── nss session A
        └── remote PTY + zsh

Terminal Tab B
  └── nss session B
        └── remote PTY + zsh
```

One native terminal tab maps to one remote session. When the connection is interrupted, `nss` retries SSH and reattaches to the original PTY when connectivity returns.

## Design boundaries

- Use the system OpenSSH client; do not reimplement SSH.
- LAN, VPN, Cloudflare Tunnel, jump hosts, and similar options are only SSH transports.
- `nss` does not intercept tmux prefixes or manage terminal tabs.
- `nssd` owns the remote PTY and shell process.
- Disconnected output uses bounded disk spool storage, not unbounded memory.
- Raw keyboard input is not saved or replayed by default while disconnected.
- A second client may not silently take over a session; takeover must be explicit.

## Current status

The repository contains a working MVP vertical slice and has:

- Project engineering guidelines and documentation entry points.
- A Go module with `nss` and `nssd` executables.
- GitHub Actions CI.
- GitHub tag releases and GoReleaser configuration.
- A checksum-verifying installer.

Implemented:

- `nssd serve`: manage the remote PTY and shell.
- `nssd attach`: forward the attach protocol over SSH stdin/stdout.
- `nssd update`: update both `nss` and `nssd` on the current machine.
- `nssd service install|status|restart|uninstall`: register and manage the background daemon service.
- `nss <host>`: use system OpenSSH with reconnect, PTY input/output, and resize support.
- `nss update`: update both `nss` and `nssd` on the current machine.
- Bounded disk spool and reconnect replay for output produced while disconnected.

Still to be completed: persistence across daemon restarts, takeover, idle cleanup, a Windows service backend, and production deployment hardening.

## Development

Go is required. Use the Go version declared by `go.mod`.

```bash
go test -race -cover ./...
go vet ./...
go build ./cmd/nss ./cmd/nssd
```

For detailed design, read:

- [Architecture](docs/architecture.md)
- [Session lifecycle](docs/lifecycle.md)
- [Attach protocol](docs/protocol.md)
- [Server operations](docs/operations.md)
- [Testing and release process](docs/testing-and-release.md)

Traditional Chinese documentation is available in [the Traditional Chinese README](README.zh-TW.md) and [the Traditional Chinese documentation index](docs/zh-TW/README.md).

## Installation

Install the latest release with:

```bash
curl -fsSL https://raw.githubusercontent.com/gaborltd/nss/main/scripts/nss_install.sh | sh
```

The installer downloads the platform archive from a `gaborltd/nss` GitHub Release, verifies SHA-256, and installs both `nss` and `nssd` into `~/.local/bin`. A future custom domain can expose the same script as `https://YOUR-DOMAIN.example/nss_install.sh`.

After installation, update both binaries directly without downloading the installer again:

```bash
nss update
nssd update
```

You can pin a version, for example `nss update --version v0.2.1`. Updating replaces the binaries but does not restart a running `nssd serve`; use launchd, systemd, or a manual restart to load the new version.
