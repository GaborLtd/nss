# Session Lifecycle

## States

```text
CREATED → ATTACHED → DETACHED → ATTACHED
                         │
                         ├── CLOSE → CLOSED
                         └── TTL  → EXPIRED
```

### `CREATED`

Session metadata exists, but the session has not successfully attached to a PTY.

### `ATTACHED`

One client holds the session attachment. A second client is not allowed to provide input by default.

### `DETACHED`

The SSH transport is gone, but the PTY, shell, and child processes remain alive. The `nss` client enters its retry loop.

### `CLOSED`

The user explicitly closes the tab, runs `nss close`, or the shell exits normally. `nssd` should clean up the PTY, metadata, and spool.

### `EXPIRED`

The session matches the cleanup policy and is reclaimed by the server.

## Cleanup principles

Do not decide that a session is removable based only on its last output time. These cases may be quiet while still working:

- A compiler or test process waiting for input.
- `sleep` or a long computation.
- A background job.
- A server process that temporarily has no logs.

The first version uses a conservative policy. Automatic expiry is allowed only when all conditions hold:

1. The client is detached.
2. The PTY has no foreground child process.
3. The shell is back at a prompt or explicitly reports idle through shell integration.
4. Input and output have exceeded the idle TTL.
5. There is no active background job, or the background job state has been reliably determined.

If the state cannot be determined, do not delete automatically; leave it to an administrative command.

The suggested initial TTL is 12–24 hours and should be adjusted after real usage testing. An attached session must not be removed for being idle.

## Attach policy

By default:

- A session has only one active attachment.
- A second attach is rejected with the current owner/attachment state.
- Automatic reconnect uses the original session secret.

Advanced management:

```bash
nss list office-mini
nss attach office-mini <session-id>
nss takeover office-mini <session-id>
nss close office-mini <session-id>
```

`takeover` must be explicit. After a successful takeover, the old attachment should receive a recognizable `taken over` state instead of silently competing with the new client.

The MVP provides `nssd list` and `nssd close --session-id`. Cross-device takeover and a more complete admin control plane remain future work.

## Crashes and restarts

- `nss` crash: the remote session remains detached until reconnect, explicit attach, or cleanup.
- `nssd` crash: recovery depends on whether metadata and the PTY still exist. The first version may mark the session lost and does not promise PTY preservation across a daemon crash.
- Server reboot: the first version does not promise to preserve running processes. launchd, checkpoints, or a higher-level mechanism may be designed later.
