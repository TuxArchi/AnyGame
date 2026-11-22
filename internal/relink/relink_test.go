// SPDX-License-Identifier: GPL-2.0-only

package relink

import (
	"reflect"
	"testing"
)

func TestArgsLayout(t *testing.T) {
	options := Options{
		Relinker: "relinker", Input: "/in/eboot.elf", Output: "/out/game.exe",
		Target: "windows", SceModulePath: "/in", Registry: true, WindowsGUI: true,
		ModuleDirs: []string{"Media/Plugins"}, Executables: []string{"picker/picker.bin"},
		Patches: "patch.toml",
	}
	got := options.Args()
	want := []string{
		"--windows", "--windows-gui", "--registry", "--rpath", "$ORIGIN/libs",
		"--sce-module-path", "/in", "--module-dir", "Media/Plugins",
		"--executable", "picker/picker.bin", "--patches", "patch.toml",
		"/in/eboot.elf", "/out/game.exe",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args =\n%v\nwant\n%v", got, want)
	}
}

func TestArgsSkipModulesConflictsWithModulePath(t *testing.T) {
	options := Options{
		Relinker: "relinker", Input: "in.elf", Output: "out.elf",
		Target: "linux", SceModulePath: "in", SkipModules: true, NoPatches: true, DryRun: true,
	}
	got := options.Args()
	for _, argument := range got {
		if argument == "--sce-module-path" {
			t.Fatalf("--skip-sce-module must replace --sce-module-path: %v", got)
		}
	}
	want := []string{"--dry-run", "--rpath", "$ORIGIN/libs", "--skip-sce-module", "--no-patches", "in.elf", "out.elf"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args =\n%v\nwant\n%v", got, want)
	}
}
