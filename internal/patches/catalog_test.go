// SPDX-License-Identifier: GPL-2.0-only

package patches

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeCatalog(t *testing.T, content []byte) (string, string) {
	t.Helper()
	root := t.TempDir()
	gameRoot := filepath.Join(root, "game")
	if err := os.MkdirAll(gameRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gameRoot, "eboot.bin"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	catalogDir := filepath.Join(root, "catalog")
	if err := os.MkdirAll(filepath.Join(catalogDir, "patches"), 0o755); err != nil {
		t.Fatal(err)
	}
	patch := "format = 1\n\n[[patch]]\nname = \"fix\"\ntarget = \"eboot.bin\"\nva = 0x1000\nexpected = \"90\"\nvalue = \"cc\"\n"
	if err := os.WriteFile(filepath.Join(catalogDir, "patches", "fix.toml"), []byte(patch), 0o644); err != nil {
		t.Fatal(err)
	}
	index := map[string]any{
		"format": 1,
		"entries": []map[string]any{{
			"titleId":        "PPSA00001",
			"contentVersion": "01.000.000",
			"patch":          "patches/fix.toml",
			"targets":        map[string]string{"eboot.bin": hex.EncodeToString(sum[:])},
			"id":             "fix",
			"name":           "Test fix",
			"category":       "graphics",
			"description":    "A test patch",
		}},
	}
	body, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(catalogDir, "index.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	return catalogDir, gameRoot
}

func TestFindCompatiblePatch(t *testing.T) {
	catalog, gameRoot := makeCatalog(t, []byte("game bytes"))
	result, err := Find(context.Background(), catalog, t.TempDir(), gameRoot, "PPSA00001", "01.000.000")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(result.Matches) != 1 {
		t.Fatalf("matches = %#v", result.Matches)
	}
	match := result.Matches[0]
	if !match.Compatible {
		t.Fatalf("patch is incompatible: %s", match.Reason)
	}
	if match.Path == "" {
		t.Fatal("patch path is empty")
	}
	if _, err := os.Stat(match.Path); err != nil {
		t.Fatalf("patch file: %v", err)
	}
	if match.Name != "Test fix" || match.Category != "graphics" {
		t.Fatalf("metadata = %#v", match)
	}
}

func TestFindRejectsDifferentDump(t *testing.T) {
	catalog, gameRoot := makeCatalog(t, []byte("game bytes"))
	if err := os.WriteFile(filepath.Join(gameRoot, "eboot.bin"), []byte("different bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Find(context.Background(), catalog, t.TempDir(), gameRoot, "PPSA00001", "01.000.000")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(result.Matches) != 1 || result.Matches[0].Compatible {
		t.Fatalf("matches = %#v", result.Matches)
	}
	if !strings.Contains(result.Matches[0].Reason, "SHA-256") {
		t.Fatalf("reason = %q", result.Matches[0].Reason)
	}
}

func TestFindFiltersTitleAndVersion(t *testing.T) {
	catalog, gameRoot := makeCatalog(t, []byte("game bytes"))
	result, err := Find(context.Background(), catalog, t.TempDir(), gameRoot, "PPSA99999", "01.000.000")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(result.Matches) != 0 {
		t.Fatalf("matches = %#v", result.Matches)
	}
}

func TestRejectsUnsafePatchPath(t *testing.T) {
	root := t.TempDir()
	body := `{"format":1,"entries":[{"titleId":"PPSA00001","contentVersion":"01.000.000","patch":"../escape.toml","targets":{}}]}`
	if err := os.WriteFile(filepath.Join(root, "index.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(context.Background(), root, t.TempDir()); err == nil {
		t.Fatal("expected an unsafe path to be rejected")
	}
}

func TestResolveGitHubRepository(t *testing.T) {
	index, base, err := resolveSource("https://github.com/owner/repo")
	if err != nil {
		t.Fatalf("resolveSource: %v", err)
	}
	if index != "https://raw.githubusercontent.com/owner/repo/HEAD/index.json" {
		t.Fatalf("index = %s", index)
	}
	if base != "https://raw.githubusercontent.com/owner/repo/HEAD" {
		t.Fatalf("base = %s", base)
	}
}
