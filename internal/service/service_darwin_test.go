//go:build darwin

package service

import (
	"errors"
	"os"
	"strconv"
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

func TestLaunchdDomainCandidatesPreferUserDomain(t *testing.T) {
	domains := launchdDomainCandidates()
	if len(domains) != 2 {
		t.Fatalf("domains = %#v, want user and gui domains", domains)
	}
	if want := "user/" + strconv.Itoa(os.Getuid()); domains[0] != want {
		t.Fatalf("first domain = %q, want %q", domains[0], want)
	}
	if want := "gui/" + strconv.Itoa(os.Getuid()); domains[1] != want {
		t.Fatalf("second domain = %q, want %q", domains[1], want)
	}
}

func TestIsUnsupportedLaunchdDomain(t *testing.T) {
	if !isUnsupportedLaunchdDomain(errors.New("exit status 125: Domain does not support specified action")) {
		t.Fatal("expected error 125 to be recognized as an unsupported domain")
	}
	if isUnsupportedLaunchdDomain(errors.New("exit status 5: Input/output error")) {
		t.Fatal("did not expect unrelated launchctl error to be recognized")
	}
}
