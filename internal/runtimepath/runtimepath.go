package runtimepath

import (
	"os"
	"path/filepath"
)

func StateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "nss"), nil
}

func SocketPath() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "nssd.sock"), nil
}
