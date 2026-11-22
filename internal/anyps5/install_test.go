// SPDX-License-Identifier: GPL-2.0-only

package anyps5

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(writer)
	for name, body := range files {
		if err := archive.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func sum(data []byte) string {
	hash := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(hash[:])
}

func releaseServer(t *testing.T, relinker, libs []byte) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var server *httptest.Server
	serveRelease := func(w http.ResponseWriter, r *http.Request) {
		release := map[string]any{
			"tag_name": "v9.9.9",
			"name":     "9.9.9",
			"html_url": server.URL + "/release",
			"assets": []map[string]any{
				{"name": "relinker-v9.9.9", "size": len(relinker), "digest": sum(relinker), "browser_download_url": server.URL + "/assets/relinker"},
				{"name": "relinker-v9.9.9.exe", "size": len(relinker), "digest": sum(relinker), "browser_download_url": server.URL + "/assets/relinker"},
				{"name": "prx-linux-v9.9.9.tar.gz", "size": len(libs), "digest": sum(libs), "browser_download_url": server.URL + "/assets/libs"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(release)
	}
	mux.HandleFunc("/repos/boykopovar/AnyPS5/releases/latest", serveRelease)
	mux.HandleFunc("/repos/boykopovar/AnyPS5/releases/tags/", serveRelease)
	mux.HandleFunc("/assets/relinker", func(w http.ResponseWriter, r *http.Request) { w.Write(relinker) })
	mux.HandleFunc("/assets/libs", func(w http.ResponseWriter, r *http.Request) { w.Write(libs) })
	server = httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func TestInstallWritesRuntimeAndState(t *testing.T) {
	home := t.TempDir()
	t.Setenv("ANYGAME_HOME", home)
	relinker := []byte("#!/bin/sh\nexit 0\n")
	libs := tarGz(t, map[string]string{
		"libs/libkernel.prx":     "kernel",
		"libs/licenses/opus.txt": "license",
	})
	server := releaseServer(t, relinker, libs)
	client := NewClient()
	client.APIBase = server.URL

	result, err := Install(context.Background(), client, "linux", "", false, nil)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if result.Version != "9.9.9" || result.AlreadyInstalled {
		t.Fatalf("unexpected result: %#v", result)
	}
	relinkerName := "relinker"
	if runtime.GOOS == "windows" {
		relinkerName = "relinker.exe"
	}
	relinkerPath := filepath.Join(home, "runtimes", "9.9.9", "linux", "bin", relinkerName)
	info, err := os.Stat(relinkerPath)
	if err != nil {
		t.Fatalf("relinker: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("relinker is not executable: %v", info.Mode())
	}
	if _, err := os.Stat(filepath.Join(home, "runtimes", "9.9.9", "linux", "libs", "libkernel.prx")); err != nil {
		t.Fatalf("library: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "runtimes", "9.9.9", "linux", "runtime.json")); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if active := ActiveVersion("linux"); active != "9.9.9" {
		t.Fatalf("active = %q", active)
	}
	runtime, err := Ensure("linux", "")
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if runtime.Relinker != relinkerPath {
		t.Fatalf("relinker = %s", runtime.Relinker)
	}
	second, err := Install(context.Background(), client, "linux", "9.9.9", false, nil)
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}
	if !second.AlreadyInstalled {
		t.Fatal("reinstall did not report an existing runtime")
	}
}

func TestInstallRejectsBadChecksum(t *testing.T) {
	home := t.TempDir()
	t.Setenv("ANYGAME_HOME", home)
	relinker := []byte("binary")
	libs := tarGz(t, map[string]string{"libs/libkernel.prx": "kernel"})
	server := releaseServer(t, relinker, libs)
	// Flip one byte of the served relinker so the digest no longer matches.
	broken := append([]byte(nil), relinker...)
	broken[0] ^= 0xff
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/boykopovar/AnyPS5/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		release := map[string]any{
			"tag_name": "v9.9.9",
			"assets": []map[string]any{
				{"name": "relinker-v9.9.9", "size": len(relinker), "digest": sum(relinker), "browser_download_url": server.URL + "/broken"},
				{"name": "relinker-v9.9.9.exe", "size": len(relinker), "digest": sum(relinker), "browser_download_url": server.URL + "/broken"},
				{"name": "prx-linux-v9.9.9.tar.gz", "size": len(libs), "digest": sum(libs), "browser_download_url": server.URL + "/libs"},
			},
		}
		json.NewEncoder(w).Encode(release)
	})
	mux.HandleFunc("/broken", func(w http.ResponseWriter, r *http.Request) { w.Write(broken) })
	mux.HandleFunc("/libs", func(w http.ResponseWriter, r *http.Request) { w.Write(libs) })
	brokenServer := httptest.NewServer(mux)
	defer brokenServer.Close()
	client := NewClient()
	client.APIBase = brokenServer.URL
	if _, err := Install(context.Background(), client, "linux", "", false, nil); err == nil {
		t.Fatal("expected a checksum mismatch")
	}
	entries, err := os.ReadDir(filepath.Join(home, "runtimes"))
	if err == nil && len(entries) > 0 {
		t.Fatalf("a failed install left runtimes behind: %v", entries)
	}
}
