// Package service 管理 nssd 的作業系統 user service。
package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Name           string
	ExecutablePath string
}

type Status struct {
	Installed bool
	Active    bool
	UnitPath  string
}

func Install(config Config) (Status, error) {
	if err := ensureNonRoot(); err != nil {
		return Status{}, err
	}
	config, err := normalizeConfig(config)
	if err != nil {
		return Status{}, err
	}
	return install(config)
}

func StatusOf(config Config) (Status, error) {
	if err := ensureNonRoot(); err != nil {
		return Status{}, err
	}
	config, err := normalizeConfig(config)
	if err != nil {
		return Status{}, err
	}
	return status(config)
}

func Restart(config Config) (Status, error) {
	if err := ensureNonRoot(); err != nil {
		return Status{}, err
	}
	config, err := normalizeConfig(config)
	if err != nil {
		return Status{}, err
	}
	return restart(config)
}

func Uninstall(config Config) (Status, error) {
	if err := ensureNonRoot(); err != nil {
		return Status{}, err
	}
	config, err := normalizeConfig(config)
	if err != nil {
		return Status{}, err
	}
	return uninstall(config)
}

func ensureNonRoot() error {
	return requireNonRoot(os.Geteuid())
}

func requireNonRoot(euid int) error {
	if euid == 0 {
		return errors.New("nssd service commands must run as the target user; do not use sudo")
	}
	return nil
}

func normalizeConfig(config Config) (Config, error) {
	if config.Name == "" {
		config.Name = "nssd"
	}
	if config.ExecutablePath == "" {
		return Config{}, errors.New("找不到 nssd executable path")
	}
	path, err := filepath.Abs(config.ExecutablePath)
	if err != nil {
		return Config{}, fmt.Errorf("解析 nssd executable path 失敗: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	if _, err := os.Stat(path); err != nil {
		return Config{}, fmt.Errorf("找不到 nssd binary %s: %w", path, err)
	}
	config.ExecutablePath = path
	return config, nil
}
