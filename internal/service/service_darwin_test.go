//go:build darwin

package service

import (
	"strings"
	"testing"
)

func TestRenderLaunchdPlistUsesAbsoluteExecutableAndLogs(t *testing.T) {
	content := string(renderLaunchdPlist("/Users/test/.local/bin/nssd", "/Users/test/Library/Logs/nss"))
	for _, expected := range []string{"com.gaborltd.nssd", "/Users/test/.local/bin/nssd", "<string>serve</string>", "nssd.stdout.log", "nssd.stderr.log"} {
		if !strings.Contains(content, expected) {
			t.Fatalf("plist missing %q: %s", expected, content)
		}
	}
}
