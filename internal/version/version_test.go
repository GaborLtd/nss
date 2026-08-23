package version

import "testing"

func TestString(t *testing.T) {
	oldVersion, oldCommit, oldDate := Version, Commit, Date
	t.Cleanup(func() {
		Version, Commit, Date = oldVersion, oldCommit, oldDate
	})

	Version = "0.1.0"
	Commit = "abc123"
	Date = "2026-08-23"

	want := "nss 0.1.0 (commit=abc123, date=2026-08-23)"
	if got := String("nss"); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
