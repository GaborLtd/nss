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

## Framing

每個 frame 使用固定 5-byte header：

```text
1 byte   type
4 bytes  payload length（big-endian uint32）
N bytes  payload
```

單一 frame 的 payload 上限為 16 MiB。PTY output 使用 `DATA` frame；control payload 使用 JSON 或固定長度 binary payload。

目前 frame type：

| Type | 用途 | Payload |
|---|---|---|
| `OPEN` | 建立或重新 attach session | JSON |
| `OPEN_OK` | attach 成功 | JSON |
| `OPEN_ERROR` | attach 失敗 | UTF-8 error message |
| `DATA` | terminal input/output | raw bytes |
| `RESIZE` | terminal resize | rows/cols，各 2 bytes |
| `CLOSE` | 明確關閉 session | empty |

## 設計原則

- protocol 必須有 version。
- control frame 與 raw PTY bytes 必須可區分。
- frame 必須有 length limit，拒絕異常的大 frame。
- attach、takeover、close 都要有明確 result code。
- reconnect 必須是 idempotent：同一 client retry 不應建立多個 PTY。
- server 必須能回報 replay gap，不能假裝 spool 保存了所有 output。

## 後續 protocol revision

以下項目在 production hardening 前必須形成 ADR 或 protocol revision：

- session secret 的傳遞與 rotation。
- replay spool 的 offset/sequence number。
- terminal resize race condition。
- takeover 時舊 client 的 shutdown handshake。
- shell integration 的 optional capability。
