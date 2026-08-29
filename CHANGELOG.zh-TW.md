# Changelog

重要變更記錄於此。版本遵循 Semantic Versioning。

## [Unreleased]

- 修正 macOS service registration 在 SSH/headless session 失敗的問題：優先使用 user launchd domain，GUI domain 可用時再 fallback。

## [0.2.2] - 2026-08-29

- 修正多行 SSH diagnostic 與 reconnect 訊息混入 interactive terminal，造成畫面看似損壞的問題。
- 新增保存 cursor 位置的 reconnect status 顯示與壓縮後的 transport diagnostic。
- 新增 reconnect 畫面行為的 regression tests 與操作文件。

- 新 session 現在會從 authenticated Unix user 的 home directory 開始，與 `ssh <host>` 的預設 working directory 一致。

## [0.2.1] - 2026-08-27

- 新增 `nss update` 與 `nssd update`；驗證 checksum 後同時更新兩個 binary。
- 新增 `nssd service install|status|restart|uninstall`，管理 macOS LaunchAgent 與 Linux systemd user service。
- 重新建立 release tag，避開 GitHub Actions incident 期間遺留的 queued runs。

## [0.1.4] - 2026-08-26

- `nssd serve` 啟動完成後顯示 ready 訊息。
- 補充使用 `nssd list` 驗證 daemon 的操作文件。
- 建立專案 bootstrap、架構、session lifecycle、protocol、GoReleaser、GitHub Actions CI 與 checksum installer。
- 完成第一個 MVP vertical slice：PTY session、SSH attach proxy、reconnect client、bounded spool 與 terminal resize。
