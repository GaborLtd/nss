# Attach Protocol 初稿

## 目的

此 protocol 是 `nss` 與 `nssd` 之間的 application protocol，承載於標準 SSH stdin/stdout。它不取代 SSH，也不處理 SSH authentication。

## 連線流程

```text
nss → ssh -T host nssd attach ...
nss ← HELLO / protocol version
nss → ATTACH(session id, secret, terminal size)
nssd → ATTACHED(session metadata)
nss ⇄ PTY_BYTES
nss → RESIZE(rows, cols)
nss → CLOSE 或 transport EOF
```

## 設計原則

- protocol 必須有 version。
- control frame 與 raw PTY bytes 必須可區分。
- frame 必須有 length limit，拒絕異常的大 frame。
- attach、takeover、close 都要有明確 result code。
- reconnect 必須是 idempotent：同一 client retry 不應建立多個 PTY。
- server 必須能回報 replay gap，不能假裝 spool 保存了所有 output。

## 尚未決定的項目

以下項目在實作前必須形成 ADR 或 protocol revision：

- binary framing 或 sideband channel。
- PTY bytes 與 control frames 的 multiplexing。
- session secret 的傳遞與 rotation。
- replay spool 的 offset/sequence number。
- terminal resize race condition。
- takeover 時舊 client 的 shutdown handshake。
- shell integration 的 optional capability。
