package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRunUpdatesExecutableFromLatestRelease(t *testing.T) {
	const repository = "owner/repo"
	asset := "nss_0.1.5_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
	archive := makeArchive(t, "nss_0.1.5_"+runtime.GOOS+"_"+runtime.GOARCH+"/nss", []byte("new binary"))
	hash := sha256.Sum256(archive)
	checksums := fmt.Sprintf("%x  %s\n", hash, asset)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/" + repository + "/releases/latest":
			http.Redirect(w, r, "/"+repository+"/releases/tag/v0.1.5", http.StatusFound)
		case "/" + repository + "/releases/tag/v0.1.5":
			w.WriteHeader(http.StatusOK)
		case "/" + repository + "/releases/download/v0.1.5/" + asset:
			_, _ = w.Write(archive)
		case "/" + repository + "/releases/download/v0.1.5/checksums.txt":
			_, _ = w.Write([]byte(checksums))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "nss")
	if err := os.WriteFile(target, []byte("old binary"), 0755); err != nil {
		t.Fatal(err)
	}
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(Config{
		Repository:     repository,
		ExecutablePath: target,
		BaseURL:        server.URL,
		HTTPClient:     server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Tag != "v0.1.5" || result.Path != resolvedTarget {
		t.Fatalf("result = %#v, want v0.1.5 and %s", result, resolvedTarget)
	}
	updated, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(updated) != "new binary" {
		t.Fatalf("updated binary = %q, want new binary", updated)
	}
}

func TestRunRejectsChecksumMismatchWithoutReplacingBinary(t *testing.T) {
	asset := "nss_0.1.5_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
	archive := makeArchive(t, "nss_0.1.5_"+runtime.GOOS+"_"+runtime.GOARCH+"/nss", []byte("new binary"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/owner/repo/releases/download/v0.1.5/" + asset:
			_, _ = w.Write(archive)
		case "/owner/repo/releases/download/v0.1.5/checksums.txt":
			_, _ = w.Write([]byte("0000000000000000000000000000000000000000000000000000000000000000  " + asset + "\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "nss")
	if err := os.WriteFile(target, []byte("old binary"), 0755); err != nil {
		t.Fatal(err)
	}
	_, err := Run(Config{
		Repository:     "owner/repo",
		Version:        "v0.1.5",
		ExecutablePath: target,
		BaseURL:        server.URL,
		HTTPClient:     server.Client(),
	})
	if err == nil {
		t.Fatal("Run succeeded with a bad checksum")
	}
	updated, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(updated) != "old binary" {
		t.Fatalf("binary changed after checksum failure: %q", updated)
	}
}

func makeArchive(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(compressed)
	if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0755, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
