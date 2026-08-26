//go:build !darwin && !linux

package service

import "errors"

func install(Config) (Status, error) {
	return Status{}, errors.New("目前只支援 macOS LaunchAgent 與 Linux systemd user service")
}

func status(Config) (Status, error) {
	return Status{}, errors.New("目前只支援 macOS LaunchAgent 與 Linux systemd user service")
}

func restart(Config) (Status, error) {
	return Status{}, errors.New("目前只支援 macOS LaunchAgent 與 Linux systemd user service")
}

func uninstall(Config) (Status, error) {
	return Status{}, errors.New("目前只支援 macOS LaunchAgent 與 Linux systemd user service")
}
