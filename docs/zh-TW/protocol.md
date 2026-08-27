# Attach Protocol 初稿

此 protocol 是 `nss` 與 `nssd` 之間的 application protocol，承載於標準 SSH stdin/stdout；它不取代 SSH，也不處理 authentication。

## Framing

每個 frame 使用固定 5-byte header：

```text
1 byte   type
4 bytes  payload length（big-endian uint32）
N bytes  payload
```

單一 payload 上限為 16 MiB。PTY output 使用 `DATA`，control payload 使用 JSON 或固定長度 binary payload。

| Type | 用途 | Payload |
|---|---|---|
| `OPEN` | 建立或重新 attach | JSON |
| `OPEN_OK` | attach 成功 | JSON |
| `OPEN_ERROR` | attach 失敗 | UTF-8 error |
| `DATA` | terminal input/output | raw bytes |
| `RESIZE` | terminal resize | rows/cols，各 2 bytes |
| `CLOSE` | 明確關閉 session | empty |
| `ADMIN_LIST` | 查詢 metadata | empty |
| `ADMIN_OK` | admin 成功或 session list | JSON 或 empty |
| `ADMIN_CLOSE` | 強制關閉 session | JSON |

## 設計原則

Protocol 必須有 version、length limit，並可區分 control frame 與 raw PTY bytes。attach、takeover、close 必須回報明確 result code。reconnect 必須 idempotent；server 必須回報 replay gap，不能假裝 spool 保存了所有 output。

後續需定義 session secret rotation、replay offset/sequence number、resize race、takeover shutdown handshake 與 optional shell integration。
