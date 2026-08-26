//go:build linux

package service

import (
	"strings"
	"testing"
)

func TestRenderSystemdUnitUsesAbsoluteExecutable(t *testing.T) {
	content := renderSystemdUnit("/home/test/.local/bin/nssd")
	for _, expected := range []string{"Description=Native Session Shell daemon", "ExecStart=/home/test/.local/bin/nssd serve", "Restart=always", "WantedBy=default.target"} {
		if !strings.Contains(content, expected) {
			t.Fatalf("unit missing %q: %s", expected, content)
		}
	}
}
