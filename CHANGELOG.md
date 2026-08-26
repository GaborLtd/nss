# Changelog

所有重要變更會記錄在這裡。版本遵循 Semantic Versioning。

## [Unreleased]

- 新增 `nss update` 與 `nssd update`，支援 checksum 驗證後更新目前 binary。
- update 使用 atomic replace，checksum 或 archive 驗證失敗時不會取代既有 binary。

## [0.1.4] - 2026-08-26

- `nssd serve` 啟動完成後顯示 ready 訊息，方便確認 Unix socket 與設定。
- 補充使用 `nssd list` 驗證 daemon 的操作文件。

- 建立 Native Session Shell（`nss`）project bootstrap。
- 建立架構、session lifecycle、protocol 與 release 文件。
- 建立 Go scaffold、GitHub Actions CI、GoReleaser 與 checksum installer。
- 完成第一個 MVP vertical slice：PTY session、SSH attach proxy、reconnect client、bounded spool 與 terminal resize。
