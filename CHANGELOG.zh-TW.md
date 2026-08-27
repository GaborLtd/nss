# Changelog

重要變更記錄於此。版本遵循 Semantic Versioning。

## [Unreleased]

## [0.2.1] - 2026-08-27

- 新增 `nss update` 與 `nssd update`；驗證 checksum 後同時更新兩個 binary。
- 新增 `nssd service install|status|restart|uninstall`，管理 macOS LaunchAgent 與 Linux systemd user service。
- 重新建立 release tag，避開 GitHub Actions incident 期間遺留的 queued runs。

## [0.1.4] - 2026-08-26

- `nssd serve` 啟動完成後顯示 ready 訊息。
- 補充使用 `nssd list` 驗證 daemon 的操作文件。
- 建立專案 bootstrap、架構、session lifecycle、protocol、GoReleaser、GitHub Actions CI 與 checksum installer。
- 完成第一個 MVP vertical slice：PTY session、SSH attach proxy、reconnect client、bounded spool 與 terminal resize。
