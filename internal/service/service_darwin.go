//go:build darwin

package service

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

const launchdLabel = "com.gaborltd.nssd"

func install(config Config) (Status, error) {
	path, err := unitPath()
	if err != nil {
		return Status{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return Status{}, fmt.Errorf("建立 LaunchAgents directory 失敗: %w", err)
	}
	logDir := filepath.Join(os.Getenv("HOME"), "Library", "Logs", "nss")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return Status{}, fmt.Errorf("建立 nss log directory 失敗: %w", err)
	}
	content := renderLaunchdPlist(config.ExecutablePath, logDir)
	if err := atomicWrite(path, content, 0600); err != nil {
		return Status{}, err
	}
	loaded := launchdLoaded()
	if !loaded {
		if err := runLaunchctl("bootstrap", launchdDomain(), path); err != nil {
			return Status{}, fmt.Errorf("註冊 LaunchAgent 失敗: %w", err)
		}
	}
	return Status{Installed: true, Active: launchdLoaded(), UnitPath: path}, nil
}

func status(config Config) (Status, error) {
	path, err := unitPath()
	if err != nil {
		return Status{}, err
	}
	installed := fileExists(path)
	return Status{Installed: installed, Active: installed && launchdLoaded(), UnitPath: path}, nil
}

func restart(config Config) (Status, error) {
	path, err := unitPath()
	if err != nil {
		return Status{}, err
	}
	if !fileExists(path) {
		return Status{}, fmt.Errorf("LaunchAgent 尚未安裝，請先執行 `nssd service install`")
	}
	if !launchdLoaded() {
		if err := runLaunchctl("bootstrap", launchdDomain(), path); err != nil {
			return Status{}, fmt.Errorf("啟動 LaunchAgent 失敗: %w", err)
		}
	} else if err := runLaunchctl("kickstart", "-k", launchdDomain()+"/"+launchdLabel); err != nil {
		return Status{}, fmt.Errorf("重啟 LaunchAgent 失敗: %w", err)
	}
	return Status{Installed: true, Active: launchdLoaded(), UnitPath: path}, nil
}

func uninstall(config Config) (Status, error) {
	path, err := unitPath()
	if err != nil {
		return Status{}, err
	}
	if launchdLoaded() {
		if err := runLaunchctl("bootout", launchdDomain()+"/"+launchdLabel); err != nil {
			return Status{}, fmt.Errorf("停止 LaunchAgent 失敗: %w", err)
		}
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return Status{}, fmt.Errorf("移除 LaunchAgent plist 失敗: %w", err)
	}
	return Status{UnitPath: path}, nil
}

func unitPath() (string, error) {
	home := os.Getenv("HOME")
	if home == "" {
		return "", fmt.Errorf("找不到 HOME")
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist"), nil
}

func launchdDomain() string {
	return "gui/" + strconv.Itoa(os.Getuid())
}

func launchdLoaded() bool {
	return exec.Command("launchctl", "print", launchdDomain()+"/"+launchdLabel).Run() == nil
}

func runLaunchctl(args ...string) error {
	command := exec.Command("launchctl", args...)
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

func renderLaunchdPlist(executablePath, logDir string) []byte {
	var buffer bytes.Buffer
	write := func(value string) {
		var escaped bytes.Buffer
		_ = xml.EscapeText(&escaped, []byte(value))
		buffer.Write(escaped.Bytes())
	}
	buffer.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	buffer.WriteString("<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n")
	buffer.WriteString("<!-- Managed by nss. Do not edit; run nssd service install. -->\n")
	buffer.WriteString("<plist version=\"1.0\"><dict>\n")
	buffer.WriteString("<key>Label</key><string>")
	write(launchdLabel)
	buffer.WriteString("</string>\n<key>ProgramArguments</key><array><string>")
	write(executablePath)
	buffer.WriteString("</string><string>serve</string></array>\n")
	buffer.WriteString("<key>RunAtLoad</key><true/><key>KeepAlive</key><true/>\n")
	buffer.WriteString("<key>StandardOutPath</key><string>")
	write(filepath.Join(logDir, "nssd.stdout.log"))
	buffer.WriteString("</string>\n<key>StandardErrorPath</key><string>")
	write(filepath.Join(logDir, "nssd.stderr.log"))
	buffer.WriteString("</string>\n</dict></plist>\n")
	return buffer.Bytes()
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
