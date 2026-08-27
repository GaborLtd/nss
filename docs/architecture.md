# Architecture

## Goal

`nss` decouples the SSH transport from the remote terminal session: the SSH connection may disappear, but the remote PTY managed by `nssd` should remain alive.

```text
┌────────────────────┐      system ssh       ┌──────────────────────────┐
│ Laptop              │ ──────────────────── │ Office Mac mini          │
│                    │                       │                          │
│ nss client          │  stdin/stdout bytes  │ nssd                     │
│ local terminal TTY  │ ◀──────────────────▶ │ session registry          │
└────────────────────┘                       │ PTY + zsh/bash            │
                                             └──────────────────────────┘
```

`nss` does not need to know whether SSH uses an office LAN, VPN, ProxyJump, or another tunnel. If the system `ssh` can establish a connection, `nss` can use the same attach protocol.

## Components

### `nss`

- Read and write the local terminal's stdin/stdout/stderr.
- Set the local TTY to raw mode and forward terminal bytes.
- Persist the session identity for the current tab.
- Start and monitor the system `ssh` process.
- Reconnect with exponential backoff after transport loss.
- Re-run `ssh -T <host> nssd attach ...`.
- Forward terminal resize and an explicit close signal.

### `nssd`

- Remain running under remote launchd or another service manager.
- Manage the session registry.
- Create a remote PTY and start the user's selected shell.
- Consume PTY output so a full PTY buffer cannot block the child process.
- Retain the PTY after an attachment disconnects.
- Store output in a quota-limited disk spool.
- Enforce session ownership, attach, takeover, and cleanup policies.

### system OpenSSH

OpenSSH handles authentication and transport only. `nssd` manages the remote PTY itself, so OpenSSH should not allocate another pseudo-terminal.

Expected attach form:

```bash
ssh -T office-mini nssd attach --session-id <id> --secret <secret>
```

## Session identity

Each session requires:

- A human-readable session ID for display and management.
- A high-entropy session secret for reconnect authorization.
- An owner identity, at minimum bound to the authenticated SSH user.
- Created, last-attached, last-input, and last-output timestamps.
- Terminal dimensions.
- Lifecycle state.

The session ID must not be used alone as an authorization credential.

## Transport and session state

Keep the two state machines separate:

```text
transport: connected / disconnected / retrying
session:   attached / detached / closing / expired
```

An SSH disconnect changes transport state and session attachment; it must not automatically mean that the session is closed.

## Output spool

While connected:

```text
PTY output → active attachment
```

While disconnected:

```text
PTY output → bounded disk spool
                 └── small in-memory tail / metadata
```

The implementation must provide:

- A maximum byte count per session.
- A maximum byte count across the server.
- Rotation or an oldest-data-drop policy.
- An `output truncated` replay marker.
- File mode `0600`.
- Spool cleanup when a session closes.

Do not promise to preserve all terminal output; the guarantee is limited to a finite replay window.

## Non-goals

- Do not implement a new SSH protocol.
- Do not replace the terminal emulator.
- Do not manage terminal tabs.
- Do not build a tmux window/pane UI.
- Do not automatically replay arbitrary raw keyboard input from a disconnected period.
- Do not make Cloudflare, a VPN, or a specific tunnel a core dependency.
