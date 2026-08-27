# Testing、CI 與 Release Process

每個功能都必須有行為文件、單元測試、必要時的 integration test、本地與 GitHub Actions CI 驗證，以及使用者可見變更的 README/release notes。

## 本地驗證

```bash
gofmt -w .
go vet ./...
go test -race -cover ./...
go build ./cmd/nss ./cmd/nssd
```

## CI 與版本

`.github/workflows/ci.yml` 在 pull request、main push 與手動觸發時執行 gofmt、vet、race tests、coverage tests 與兩個 binary build；CI 只需 `contents: read`。

版本使用 Semantic Versioning：`vMAJOR.MINOR.PATCH`。MAJOR 是不相容變更，MINOR 是向後相容功能，PATCH 是 bug fix、文件或非行為性修正。

## Release 流程

1. 確認 main branch CI 通過。
2. 更新 changelog 與 migration 文件。
3. 建立並 push annotated tag，例如：

   ```bash
   git tag -a v0.2.1 -m "nss v0.2.1"
   git push origin v0.2.1
   ```

4. GitHub Actions 觸發 GoReleaser。
5. 確認 GitHub Release 包含 `nss`、`nssd`、macOS/Linux amd64/arm64 archives 與 `checksums.txt`。
6. 驗證 installer：

   ```bash
   NSS_REPOSITORY=gaborltd/nss NSS_VERSION=v0.2.1 sh scripts/nss_install.sh
   ```

Release job 只在 tag build 通過後執行，並使用最小的 `contents: write` permission。

## Manual test

在測試 server 啟動 `nssd serve`，用 `nss office-mini` 執行 `sleep 120`，暫時中斷 laptop 網路，確認 tab 保持開啟；網路恢復後應回到同一個 shell。也要驗證斷線期間輸入沒有被重送。

Service test 應依序執行 `nssd service status/install/status/restart/uninstall/status`，確認 service file 使用 absolute `nssd` path，且不使用 root。

## Installer contract

installer 支援 macOS/Linux、amd64/arm64，預設安裝到 `~/.local/bin`，支援 `NSS_INSTALL_DIR`、`NSS_VERSION` 與 `NSS_REPOSITORY`，透過 GitHub Releases redirect 解析最新版本，下載後驗證 SHA-256，不使用 root，也不自動修改 shell profile。
