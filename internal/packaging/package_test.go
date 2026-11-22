// SPDX-License-Identifier: GPL-2.0-only

package packaging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"anygame/internal/anyps5"
	"anygame/internal/game"
)

func makeRuntime(t *testing.T, libraries ...string) *anyps5.Runtime {
	t.Helper()
	dir := t.TempDir()
	libs := filepath.Join(dir, "libs")
	if err := os.MkdirAll(libs, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range libraries {
		if err := os.WriteFile(filepath.Join(libs, name), []byte("library "+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	relinker := filepath.Join(dir, "relinker")
	if err := os.WriteFile(relinker, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return &anyps5.Runtime{
		Version: "9.9.9", Target: "linux", Dir: dir, Relinker: relinker, LibsDir: libs,
		Manifest: anyps5.Manifest{
			Relinker: anyps5.AssetRef{Name: "relinker-v9.9.9"},
			Libs:     anyps5.AssetRef{Name: "prx-linux-v9.9.9.tar.gz"},
		},
	}
}

func makeTitle(t *testing.T) *game.Info {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sce_sys"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sce_module"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	eboot := filepath.Join(root, "eboot.bin")
	if err := os.WriteFile(eboot, []byte{0x7f, 'E', 'L', 'F'}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sce_module", "module.prx"), []byte("module"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "data", "asset.bin"), []byte("asset"), 0o644); err != nil {
		t.Fatal(err)
	}
	icon := filepath.Join(root, "sce_sys", "icon0.png")
	if err := os.WriteFile(icon, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	return &game.Info{
		Root: root, Resources: root, Eboot: eboot, Kind: game.KindELF, EbootSize: 4,
		TitleID: "PPSA00001", TitleName: "AnyGame Test", ContentVersion: "01.000.000",
		ModuleDirs: []string{filepath.Join(root, "sce_module")}, Icon: icon,
	}
}

func TestPackageCopiesOnlyNeededLibraries(t *testing.T) {
	runtime := makeRuntime(t, "libkernel.prx", "libc.prx", "libSceAudio.prx")
	title := makeTitle(t)
	output := t.TempDir()
	executable := filepath.Join(output, "PPSA00001.elf")
	if err := os.WriteFile(executable, []byte("exe"), 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := Package(Options{
		OutputDir: output, Executable: executable, Name: "PPSA00001", Target: "linux",
		Runtime: runtime, Game: title, Needed: []string{"libkernel.prx", "libc.prx"}, NeededKnown: true,
		Resources: ResourcesCopy,
	})
	if err != nil {
		t.Fatalf("Package: %v", err)
	}
	libs, err := os.ReadDir(filepath.Join(output, "libs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(libs) != 2 {
		t.Fatalf("copied %d libraries, want 2: %v", len(libs), result.Libraries)
	}
	if _, err := os.Stat(filepath.Join(output, "libs", "libSceAudio.prx")); err == nil {
		t.Fatal("copied a library that was not needed")
	}
}

func TestPackageAssemblesApp0(t *testing.T) {
	runtime := makeRuntime(t, "libkernel.prx")
	title := makeTitle(t)
	output := t.TempDir()
	executable := filepath.Join(output, "PPSA00001.elf")
	if err := os.WriteFile(executable, []byte("exe"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Package(Options{
		OutputDir: output, Executable: executable, Name: "PPSA00001", Target: "linux",
		Runtime: runtime, Game: title, Needed: []string{"libkernel.prx"}, NeededKnown: true, Resources: ResourcesCopy,
	}); err != nil {
		t.Fatalf("Package: %v", err)
	}
	if _, err := os.Stat(filepath.Join(output, "app0", "data", "asset.bin")); err != nil {
		t.Fatalf("resource missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(output, "app0", "eboot.bin")); err != nil {
		t.Fatalf("original eboot missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(output, "app0", "sce_module")); err == nil {
		t.Fatal("original module directory must not be copied; the relinker writes the converted one")
	}
	if _, err := os.Stat(filepath.Join(output, "run.sh")); err != nil {
		t.Fatalf("launcher missing: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(output, "anygame.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest Manifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Title.ID != "PPSA00001" || manifest.Executable != "PPSA00001.elf" {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	if len(manifest.Libraries) != 1 || manifest.Libraries[0] != "libkernel.prx" {
		t.Fatalf("manifest libraries = %#v", manifest.Libraries)
	}
	if !strings.Contains(string(body), "\"libraries\": [") {
		t.Fatalf("libraries is not a JSON array: %s", body)
	}
}

func TestPackageWindowsCopiesRuntimeDLLs(t *testing.T) {
	runtime := makeRuntime(t, "libkernel.prx", "libstdc++-6.dll", "libgcc_s_seh-1.dll")
	output := t.TempDir()
	executable := filepath.Join(output, "PPSA00001.exe")
	if err := os.WriteFile(executable, []byte("exe"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Package(Options{
		OutputDir: output, Executable: executable, Name: "PPSA00001", Target: "windows",
		Runtime: runtime, Needed: []string{"libkernel.prx"}, NeededKnown: true, Resources: ResourcesNone,
	}); err != nil {
		t.Fatalf("Package: %v", err)
	}
	for _, name := range []string{"libkernel.prx", "libstdc++-6.dll", "libgcc_s_seh-1.dll"} {
		if _, err := os.Stat(filepath.Join(output, "libs", name)); err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(output, "run.bat")); err != nil {
		t.Fatalf("run.bat missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(output, "run.sh")); err == nil {
		t.Fatal("windows package must not contain run.sh")
	}
}
