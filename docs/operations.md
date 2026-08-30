# Server Operations

## Manual startup

After installing `nssd` on the remote server, start the daemon as the current user:

```bash
nssd serve
```

Default paths:

```text
socket:    ~/.local/state/nss/nssd.sock
state dir: ~/.local/state/nss/
spool:     4 MiB per session
```

Override them when needed:

```bash
nssd serve \
  --socket ~/.local/state/nss/nssd.sock \
  --state-dir ~/.local/state/nss \
  --max-spool-mb 16
```

Do not run `nssd` as root. The socket and session state should be accessible only to that Unix user.

## Service management

You do not need to write a plist or systemd unit by hand. Run these commands as the current Unix user:

```bash
nssd service install
nssd service status
nssd service restart
nssd service uninstall
```

Do not run these commands with `sudo`. They register a user-level service for the current account. Running with `sudo` changes the launchd/systemd user context and can leave a root-owned service file in the user's home directory.

If `sudo` was used accidentally on macOS, repair the generated file and reinstall it as the target user:

```bash
sudo chown "$(id -un):$(id -gn)" "$HOME/Library/LaunchAgents/com.gaborltd.nssd.plist"
sudo chmod 600 "$HOME/Library/LaunchAgents/com.gaborltd.nssd.plist"
nssd service install
```

`install` generates and registers a user-level service for the operating system: a macOS LaunchAgent or a Linux systemd user service. On macOS, nss marks the LaunchAgent for both `Background` and `Aqua` sessions, prefers the `user/<uid>` launchd domain because it works from SSH and headless sessions, and falls back to `gui/<uid>` when necessary. It also recognizes existing services in either domain. `status` reports `active`, `inactive`, or `not-installed`, together with the service file path. These commands do not use root and do not manage another Unix user's daemon.

On a Linux server, enable user lingering if the service must remain alive after logout:

```bash
loginctl enable-linger "$USER"
```

The Windows service backend is not implemented. The current release supports daemon service management on macOS and Linux.

After successful startup, the foreground daemon prints a readiness message to stderr:

```text
nssd: ready; socket=/Users/developer/.local/state/nss/nssd.sock; state-dir=/Users/developer/.local/state/nss; max-spool=4 MiB
```

`nssd: ready` means the Unix socket exists, the state directory is initialized, and the daemon is waiting for an `nss` attach. The process does not return to the shell prompt; press `Ctrl-C` to stop a foreground daemon.

Each new session starts in the authenticated Unix user's home directory, matching the default working directory of `ssh <host>`. The daemon's own service working directory does not change the session's initial directory.

From another terminal, verify that the daemon accepts management requests:

```bash
nssd list
```

If the daemon is healthy but no session exists, the output contains only the header:

```text
SESSION_ID\tATTACHED
```

## macOS launchd

`nssd service install` generates `~/Library/LaunchAgents/com.gaborltd.nssd.plist` using the absolute path of the current `nssd` binary. Standard output and error are written to `~/Library/Logs/nss/`. Do not edit the generated plist by hand; run `nssd service install` again to regenerate it.

For manual inspection, the plist has this shape:

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
  <key>LimitLoadToSessionType</key>
  <array>
    <string>Background</string>
    <string>Aqua</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>/Users/developer/Library/Logs/nss/nssd.stdout.log</string>
  <key>StandardErrorPath</key>
  <string>/Users/developer/Library/Logs/nss/nssd.stderr.log</string>
</dict>
</plist>
```

The CLI performs the equivalent load/restart operations. If manual loading is required:

```bash
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.gaborltd.nssd.plist
launchctl kickstart -k gui/$(id -u)/com.gaborltd.nssd
```

## Linux systemd user service

`nssd service install` generates `~/.config/systemd/user/nssd.service`. The unit has this shape:

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

The CLI automatically runs the equivalent commands:

```bash
systemctl --user daemon-reload
systemctl --user enable --now nssd.service
```

## SSH smoke test

Verify the server daemon and its path:

```bash
ssh -T office-mini nssd --version
ssh -T office-mini nssd attach
```

Before executing the remote SSH command, `nss` adds `~/.local/bin` to the remote `PATH`. Therefore the default installer location works for non-interactive SSH shells. The `nssd` binary must be installed on the remote machine and the daemon must already be running:

```bash
# Run this on the remote server.
nssd serve
```

If you still see `command not found: nssd`, check the remote server:

```bash
command -v nssd
~/.local/bin/nssd --version
```

If `~/.local/bin/nssd` exists but `command -v nssd` does not find it, the remote installation is incomplete or uses a different install path.

The second SSH command waits for protocol bytes and is not intended for an interactive terminal. It is mainly useful for checking the binary and socket path. For normal use, run:

```bash
nss office-mini
```

During a normal interactive session, SSH diagnostic output is captured instead of being written directly into the remote PTY view. If the transport disconnects, `nss` renders a short cursor-preserving reconnect status line and clears it before the next connection attempt. This keeps the remote prompt and terminal layout readable while still showing that reconnect is in progress.

`nss` explicitly passes the current user's `~/.ssh/config` to OpenSSH when that file exists, so host aliases such as `mdev3` use the same configuration as `/usr/bin/ssh mdev3`.

### SSH passphrase prompts

During an interactive connection or reconnect, if OpenSSH requests a key passphrase, `nss` displays the prompt in the current terminal. Type the passphrase and press Enter; input is sent only to the SSH authentication process and is not stored in the session spool or logs. Backspace and `Ctrl-C` are supported.

Non-interactive mode (`--no-tty`) cannot display a prompt. Load the key into an SSH agent before using it:

```bash
ssh-add ~/.ssh/id_ed25519
nss --no-tty office-mini
```

## Session management

Management commands use the same Unix socket and are available only to that Unix user:

```bash
nssd list
nssd close --session-id <session-id>

# The same management commands can run over ordinary SSH.
ssh -T office-mini nssd list
ssh -T office-mini nssd close --session-id <session-id>
```

`nssd close` terminates the selected PTY and child process. Cross-device takeover is not currently available, preventing two terminals from writing to the same shell.

## Updating binaries

After installation, update the current machine directly from GitHub Releases:

```bash
# Laptop / client.
nss update

# Remote server / daemon host.
nssd update
```

Both commands update `nss` and `nssd` together. Use `--version vX.Y.Z` to pin a version. The update downloads the platform archive, verifies `checksums.txt`, and atomically replaces the binaries. It does not restart a running `nssd serve`; restart the launchd or systemd service after updating.

## Current limitations

- Existing PTYs are not currently promised to survive a daemon restart.
- Idle cleanup and cross-device takeover are not implemented.
- The spool is a finite replay window, not unlimited terminal history.
- `nss` does not save raw keyboard input while disconnected by default.
- `kill -9` cannot run cleanup. If it leaves the terminal in raw mode, run `stty sane` or open a new terminal tab. Normal `SIGTERM`, `SIGINT`, and `SIGHUP` handling restores the local terminal state first.
