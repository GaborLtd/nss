package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRequireNonRoot(t *testing.T) {
	if err := requireNonRoot(501); err != nil {
		t.Fatalf("requireNonRoot(non-root) = %v, want nil", err)
	}
	err := requireNonRoot(0)
	if err == nil || !strings.Contains(err.Error(), "do not use sudo") {
		t.Fatalf("requireNonRoot(root) = %v, want a sudo guidance error", err)
	}
}

func TestNormalizeConfigResolvesExecutable(t *testing.T) {
	target := filepath.Join(t.TempDir(), "nssd")
	if err := os.WriteFile(target, []byte("test binary"), 0755); err != nil {
		t.Fatal(err)
	}
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	config, err := normalizeConfig(Config{ExecutablePath: target})
	if err != nil {
		t.Fatal(err)
	}
	if config.Name != "nssd" || config.ExecutablePath != resolvedTarget {
		t.Fatalf("normalized config = %#v", config)
	}
}

func TestStatusReportsMissingServiceWithoutStartingIt(t *testing.T) {
	// 使用隔離的 HOME，避免本機已安裝的 service 影響測試結果。
	t.Setenv("HOME", t.TempDir())
	target := filepath.Join(t.TempDir(), "nssd")
	if err := os.WriteFile(target, []byte("test binary"), 0755); err != nil {
		t.Fatal(err)
	}
	status, err := StatusOf(Config{ExecutablePath: target})
	if err != nil {
		t.Fatal(err)
	}
	if status.Installed || status.Active {
		t.Fatalf("status = %#v, want not installed and inactive", status)
	}
	if status.UnitPath == "" {
		t.Fatal("status has empty unit path")
	}
}
