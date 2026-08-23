# Testing、CI 與 Release Process

## Feature definition of done

任何功能完成前，必須具備：

1. 行為文件或 protocol/lifecycle 更新。
2. 單元測試。
3. 必要時的 integration test，例如 local SSH/PTY fixture。
4. 本地與 GitHub Actions CI 通過。
5. 若是使用者可見變更，更新 README 或 release notes。

## 本地驗證

```bash
gofmt -w .
go vet ./...
go test -race -cover ./...
go build ./cmd/nss ./cmd/nssd
```

release 前可使用 GoReleaser snapshot：

```bash
goreleaser release --snapshot --clean
```

## CI

`.github/workflows/ci.yml` 在 pull request、主要分支 push 與手動觸發時執行：

- gofmt check
- `go vet`
- race-enabled tests
- coverage-enabled tests
- `nss` 與 `nssd` build

CI 只需要 `contents: read`。

## Versioning

使用 Semantic Versioning：

```text
vMAJOR.MINOR.PATCH
```

- MAJOR：不相容的 protocol、CLI 或 session semantics。
- MINOR：向後相容的新功能。
- PATCH：bug fix、文件與非行為性修正。

## Release 流程

1. 確認 main branch CI 通過。
2. 更新 CHANGELOG 與必要的 migration 文件。
3. 建立並 push annotated tag：

   ```bash
   git tag -a v0.1.0 -m "nss v0.1.0"
   git push origin v0.1.0
   ```

4. GitHub Actions 觸發 GoReleaser。
5. GoReleaser 建立 GitHub Release 與 artifacts：
   - `nss_<version>_<os>_<arch>.tar.gz`
   - `checksums.txt`
6. 確認 GitHub Release assets 與 checksum。
7. 驗證 installer：

   ```bash
   NSS_REPOSITORY=gaborltd/nss NSS_VERSION=v0.1.0 sh scripts/nss_install.sh
   ```

8. 初期可使用 GitHub raw URL；有正式網域後再將 `/nss_install.sh` 指向同一支 script。

GitHub Actions release job 只在 tag build 通過後執行，並使用最小的 `contents: write` permission 建立 release。GitHub Actions 的 token permission 應明確宣告，不依賴 repository 預設值。

## Installer contract

installer 必須：

- 支援 macOS/Linux。
- 支援 amd64/arm64。
- 預設安裝到 `~/.local/bin`。
- 允許 `NSS_INSTALL_DIR` 覆寫。
- 允許 `NSS_VERSION` 安裝指定版本。
- 允許 `NSS_REPOSITORY` 指定 GitHub repository。
- 下載後驗證 SHA-256。
- 不使用 root，不自動修改 shell profile。
- 安裝完成後提示使用者確認 PATH。
