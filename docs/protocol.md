# Attach Protocol Draft

## Purpose

This protocol is the application protocol between `nss` and `nssd`, carried over standard SSH stdin/stdout. It does not replace SSH or handle SSH authentication.

## Connection flow

```text
nss → ssh -T host nssd attach ...
nss ← HELLO / protocol version
nss → ATTACH(session id, secret, terminal size)
nssd → ATTACHED(session metadata)
nss ⇄ PTY_BYTES
nss → RESIZE(rows, cols)
nss → CLOSE or transport EOF
```

## Framing

Each frame uses a fixed 5-byte header:

```text
1 byte   type
4 bytes  payload length (big-endian uint32)
N bytes  payload
```

The payload limit for one frame is 16 MiB. PTY output uses `DATA` frames; control payloads use JSON or fixed-length binary payloads.

Current frame types:

| Type | Purpose | Payload |
|---|---|---|
| `OPEN` | Create or reattach a session | JSON |
| `OPEN_OK` | Attach succeeded | JSON |
| `OPEN_ERROR` | Attach failed | UTF-8 error message |
| `DATA` | Terminal input/output | Raw bytes |
| `RESIZE` | Terminal resize | Rows/cols, 2 bytes each |
| `CLOSE` | Explicitly close the session | Empty |
| `ADMIN_LIST` | Query session metadata | Empty |
| `ADMIN_OK` | Admin success or session list | JSON or empty |
| `ADMIN_CLOSE` | Force-close a session | JSON |

## Design principles

- The protocol must have a version.
- Control frames and raw PTY bytes must be distinguishable.
- Frames must have a length limit and reject abnormally large frames.
- Attach, takeover, and close must have explicit result codes.
- Reconnect must be idempotent: retrying the same client must not create multiple PTYs.
- The server must report replay gaps rather than pretending that the spool preserved all output.

## Future protocol revisions

Before production hardening, the following must be captured in an ADR or protocol revision:

- Session-secret transport and rotation.
- Replay-spool offsets/sequence numbers.
- Terminal-resize race conditions.
- Shutdown handshake for the old client during takeover.
- Optional shell-integration capability.
