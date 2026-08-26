//go:build linux

package service

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const systemdUnitName = "nssd.service"

func install(config Config) (Status, error) {
	path, err := unitPath()
	if err != nil {
		return Status{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return Status{}, fmt.Errorf("建立 systemd user directory 失敗: %w", err)
	}
	if err := atomicWrite(path, []byte(renderSystemdUnit(config.ExecutablePath)), 0600); err != nil {
		return Status{}, err
	}
	if err := runSystemctl("--user", "daemon-reload"); err != nil {
		return Status{}, fmt.Errorf("systemd daemon-reload 失敗: %w", err)
	}
	if err := runSystemctl("--user", "enable", "--now", systemdUnitName); err != nil {
		return Status{}, fmt.Errorf("啟用 nssd systemd service 失敗: %w", err)
	}
	return Status{Installed: true, Active: systemdActive(), UnitPath: path}, nil
}

func status(config Config) (Status, error) {
	path, err := unitPath()
	if err != nil {
		return Status{}, err
	}
	installed := fileExists(path)
	return Status{Installed: installed, Active: installed && systemdActive(), UnitPath: path}, nil
}

func restart(config Config) (Status, error) {
	path, err := unitPath()
	if err != nil {
		return Status{}, err
	}
	if !fileExists(path) {
		return Status{}, fmt.Errorf("systemd service 尚未安裝，請先執行 `nssd service install`")
	}
	if err := runSystemctl("--user", "restart", systemdUnitName); err != nil {
		return Status{}, fmt.Errorf("重啟 nssd systemd service 失敗: %w", err)
	}
	return Status{Installed: true, Active: systemdActive(), UnitPath: path}, nil
}

func uninstall(config Config) (Status, error) {
	path, err := unitPath()
	if err != nil {
		return Status{}, err
	}
	if fileExists(path) {
		if err := runSystemctl("--user", "disable", "--now", systemdUnitName); err != nil {
			return Status{}, fmt.Errorf("停止 nssd systemd service 失敗: %w", err)
		}
		if err := os.Remove(path); err != nil {
			return Status{}, fmt.Errorf("移除 systemd service file 失敗: %w", err)
		}
		if err := runSystemctl("--user", "daemon-reload"); err != nil {
			return Status{}, fmt.Errorf("systemd daemon-reload 失敗: %w", err)
		}
	}
	return Status{UnitPath: path}, nil
}

func unitPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("取得 user config directory 失敗: %w", err)
	}
	return filepath.Join(configDir, "systemd", "user", systemdUnitName), nil
}

func systemdActive() bool {
	return exec.Command("systemctl", "--user", "is-active", "--quiet", systemdUnitName).Run() == nil
}

func runSystemctl(args ...string) error {
	command := exec.Command("systemctl", args...)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		if message := bytes.TrimSpace(stderr.Bytes()); len(message) > 0 {
			return fmt.Errorf("%w: %s", err, message)
		}
		return err
	}
	return nil
}

func renderSystemdUnit(executablePath string) string {
	return "[Unit]\n" +
		"Description=Native Session Shell daemon\n" +
		"After=default.target\n\n" +
		"[Service]\n" +
		"ExecStart=" + systemdEscape(executablePath) + " serve\n" +
		"Restart=always\n" +
		"RestartSec=2\n\n" +
		"[Install]\n" +
		"WantedBy=default.target\n"
}

func systemdEscape(path string) string {
	var builder strings.Builder
	for _, character := range path {
		switch character {
		case '\\':
			builder.WriteString("\\\\")
		case ' ':
			builder.WriteString("\\x20")
		case '"':
			builder.WriteString("\\x22")
		default:
			builder.WriteRune(character)
		}
	}
	return builder.String()
}

func atomicWrite(path string, content []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".nss-service-*")
	if err != nil {
		return fmt.Errorf("建立 service temporary file 失敗: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("設定 service file 權限失敗: %w", err)
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("寫入 service file 失敗: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("同步 service file 失敗: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("關閉 service file 失敗: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("安裝 service file 失敗: %w", err)
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
