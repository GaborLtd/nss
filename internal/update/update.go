// Package update 提供從 GitHub Release 更新目前執行檔的功能。
package update

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	defaultRepository = "gaborltd/nss"
	defaultBaseURL    = "https://github.com"
	maxBinarySize     = 128 << 20
)

type Config struct {
	Repository     string
	Version        string
	ExecutablePath string
	BaseURL        string
	HTTPClient     *http.Client
}

type Result struct {
	Tag   string
	Paths []string
}

func Run(config Config) (Result, error) {
	repository := config.Repository
	if repository == "" {
		repository = defaultRepository
	}
	if err := validateRepository(repository); err != nil {
		return Result{}, err
	}

	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	baseURL := strings.TrimRight(config.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	tag := config.Version
	if tag == "" || tag == "latest" {
		var err error
		tag, err = latestTag(client, baseURL, repository)
		if err != nil {
			return Result{}, err
		}
	}
	tag, releaseVersion, err := normalizeTag(tag)
	if err != nil {
		return Result{}, err
	}

	executablePath, err := resolveExecutablePath(config.ExecutablePath)
	if err != nil {
		return Result{}, err
	}
	if _, err := os.Stat(executablePath); err != nil {
		return Result{}, fmt.Errorf("找不到要更新的 binary %s: %w", executablePath, err)
	}
	currentName := filepath.Base(executablePath)
	if currentName != "nss" && currentName != "nssd" {
		return Result{}, fmt.Errorf("目前 binary 名稱必須是 nss 或 nssd：%s", currentName)
	}
	companionName := "nss"
	if currentName == "nss" {
		companionName = "nssd"
	}
	companionPath := filepath.Join(filepath.Dir(executablePath), companionName)

	asset := fmt.Sprintf("nss_%s_%s_%s.tar.gz", releaseVersion, runtime.GOOS, runtime.GOARCH)
	tempDir, err := os.MkdirTemp("", "nss-update-")
	if err != nil {
		return Result{}, fmt.Errorf("建立 update temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	baseReleaseURL := fmt.Sprintf("%s/%s/releases/download/%s", baseURL, repository, url.PathEscape(tag))
	archivePath := filepath.Join(tempDir, asset)
	checksumsPath := filepath.Join(tempDir, "checksums.txt")
	if err := downloadFile(client, baseReleaseURL+"/"+asset, archivePath); err != nil {
		return Result{}, fmt.Errorf("下載 %s 失敗: %w", asset, err)
	}
	if err := downloadFile(client, baseReleaseURL+"/checksums.txt", checksumsPath); err != nil {
		return Result{}, fmt.Errorf("下載 checksum 失敗: %w", err)
	}
	expected, err := checksumFor(checksumsPath, asset)
	if err != nil {
		return Result{}, err
	}
	actual, err := sha256File(archivePath)
	if err != nil {
		return Result{}, fmt.Errorf("計算 checksum 失敗: %w", err)
	}
	if actual != expected {
		return Result{}, fmt.Errorf("checksum 驗證失敗：expected=%s actual=%s", expected, actual)
	}

	paths := []string{executablePath, companionPath}
	for _, path := range paths {
		if err := replaceFromArchive(archivePath, path, filepath.Base(path)); err != nil {
			return Result{}, err
		}
	}
	return Result{Tag: tag, Paths: paths}, nil
}

func latestTag(client *http.Client, baseURL, repository string) (string, error) {
	requestURL := fmt.Sprintf("%s/%s/releases/latest", strings.TrimRight(baseURL, "/"), repository)
	response, err := client.Get(requestURL)
	if err != nil {
		return "", fmt.Errorf("無法取得最新 release：%w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("無法取得最新 release：HTTP %s", response.Status)
	}
	marker := "/releases/tag/"
	path := response.Request.URL.Path
	index := strings.LastIndex(path, marker)
	if index < 0 {
		return "", fmt.Errorf("無法從 release URL 解析版本：%s", response.Request.URL.String())
	}
	tag, err := url.PathUnescape(strings.Trim(path[index+len(marker):], "/"))
	if err != nil || tag == "" || strings.ContainsAny(tag, "/\\") {
		return "", fmt.Errorf("無法從 release URL 解析版本：%s", response.Request.URL.String())
	}
	return tag, nil
}

func normalizeTag(tag string) (string, string, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" || strings.ContainsAny(tag, "/\\") {
		return "", "", errors.New("release version 不合法")
	}
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}
	return tag, strings.TrimPrefix(tag, "v"), nil
}

func validateRepository(repository string) error {
	parts := strings.Split(repository, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.ContainsAny(repository, "\\?#") {
		return fmt.Errorf("GitHub repository 不合法：%s", repository)
	}
	return nil
}

func resolveExecutablePath(path string) (string, error) {
	if path == "" {
		var err error
		path, err = os.Executable()
		if err != nil {
			return "", fmt.Errorf("取得目前 executable path 失敗: %w", err)
		}
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("解析 executable path 失敗: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return path, nil
}

func downloadFile(client *http.Client, source, destination string) error {
	response, err := client.Get(source)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s", response.Status)
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, response.Body)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func checksumFor(path, name string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("開啟 checksum 失敗: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && strings.TrimPrefix(fields[1], "*") == name {
			if len(fields[0]) != sha256.Size*2 {
				return "", fmt.Errorf("checksum 格式不合法：%s", scanner.Text())
			}
			return strings.ToLower(fields[0]), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("讀取 checksum 失敗: %w", err)
	}
	return "", fmt.Errorf("checksum 中找不到 %s", name)
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func replaceFromArchive(archivePath, targetPath, binaryName string) error {
	archive, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("開啟 release archive 失敗: %w", err)
	}
	defer archive.Close()
	compressed, err := gzip.NewReader(archive)
	if err != nil {
		return fmt.Errorf("讀取 release archive 失敗: %w", err)
	}
	defer compressed.Close()

	targetDir := filepath.Dir(targetPath)
	temporary, err := os.CreateTemp(targetDir, "."+filepath.Base(targetPath)+".update-*")
	if err != nil {
		return fmt.Errorf("建立 binary temporary file 失敗: %w", err)
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()

	found := false
	reader := tar.NewReader(compressed)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			_ = temporary.Close()
			return fmt.Errorf("讀取 release archive entry 失敗: %w", err)
		}
		cleanName := filepath.ToSlash(filepath.Clean(header.Name))
		if filepath.IsAbs(cleanName) || cleanName == "." || strings.HasPrefix(cleanName, "../") || strings.Contains(cleanName, "/../") {
			_ = temporary.Close()
			return fmt.Errorf("release archive 含有不安全 path：%s", header.Name)
		}
		if header.Typeflag != tar.TypeReg || filepath.Base(cleanName) != binaryName {
			continue
		}
		if found {
			_ = temporary.Close()
			return fmt.Errorf("release archive 含有重複 binary：%s", binaryName)
		}
		if header.Size < 0 || header.Size > maxBinarySize {
			_ = temporary.Close()
			return fmt.Errorf("binary size 不合法：%d", header.Size)
		}
		if _, err := io.CopyN(temporary, reader, header.Size); err != nil {
			_ = temporary.Close()
			return fmt.Errorf("extract %s 失敗: %w", binaryName, err)
		}
		found = true
	}
	if !found {
		_ = temporary.Close()
		return fmt.Errorf("release archive 缺少 %s binary", binaryName)
	}
	if err := temporary.Chmod(0755); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("設定 binary 權限失敗: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("同步 binary 失敗: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("關閉 binary temporary file 失敗: %w", err)
	}
	if err := os.Rename(temporaryPath, targetPath); err != nil {
		return fmt.Errorf("取代 binary 失敗: %w", err)
	}
	removeTemporary = false
	return nil
}
