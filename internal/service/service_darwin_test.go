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

func TestBootstrapLaunchAgentFallsBackToGUIDomain(t *testing.T) {
	domains := launchdDomainCandidates()
	var calls [][]string
	err := bootstrapLaunchAgentWith(func(args ...string) error {
		calls = append(calls, append([]string(nil), args...))
		if args[1] == domains[0] {
			return errors.New("exit status 5: Input/output error")
		}
		return nil
	}, "/tmp/com.gaborltd.nssd.plist")
	if err != nil {
		t.Fatalf("bootstrapLaunchAgentWith() = %v, want success after fallback", err)
	}
	if len(calls) != 2 {
		t.Fatalf("launchctl calls = %#v, want one call per domain", calls)
	}
	if calls[0][0] != "bootstrap" || calls[0][1] != domains[0] {
		t.Fatalf("first launchctl call = %#v, want user domain bootstrap", calls[0])
	}
	if calls[1][0] != "bootstrap" || calls[1][1] != domains[1] {
		t.Fatalf("second launchctl call = %#v, want GUI domain bootstrap", calls[1])
	}
}

func TestBootstrapLaunchAgentReportsBothDomainErrors(t *testing.T) {
	err := bootstrapLaunchAgentWith(func(args ...string) error {
		return errors.New("exit status 5: Input/output error")
	}, "/tmp/com.gaborltd.nssd.plist")
	message := err.Error()
	for _, domain := range launchdDomainCandidates() {
		if !strings.Contains(message, domain) {
			t.Fatalf("error = %q, missing domain %q", message, domain)
		}
	}
}
