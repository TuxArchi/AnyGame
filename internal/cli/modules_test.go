// SPDX-License-Identifier: GPL-2.0-only

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"anygame/internal/game"
)

func TestStageModulesCopiesELFModulesAndDetectsExtras(t *testing.T) {
	root := t.TempDir()
	writeModule(t, filepath.Join(root, "sce_module", "libc.prx"))
	writeModule(t, filepath.Join(root, "Media", "Modules", "PS5Util.prx"))
	writeModule(t, filepath.Join(root, "Media", "Plugins", "libfmod.prx"))
	info := &game.Info{
		Resources:       root,
		ModuleDirs:      []string{filepath.Join(root, "sce_module")},
		ExtraModuleDirs: []string{"Media/Modules", "Media/Plugins"},
	}
	staged, extra, warnings, err := stageModules(info, root, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("stageModules: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings: %#v", warnings)
	}
	if len(extra) != 2 || extra[0] != "Media/Modules" || extra[1] != "Media/Plugins" {
		t.Fatalf("extra = %#v", extra)
	}
	for _, name := range []string{"sce_module/libc.prx", "Media/Modules/PS5Util.prx", "Media/Plugins/libfmod.prx"} {
		if _, err := os.Stat(filepath.Join(staged, filepath.FromSlash(name))); err != nil {
			t.Fatalf("%s not staged: %v", name, err)
		}
	}
	exclusions := moduleExclusions(info)
	if len(exclusions) != 3 {
		t.Fatalf("exclusions = %#v", exclusions)
	}
}

func TestStageModulesSkipsNonModules(t *testing.T) {
	root := t.TempDir()
	writeModuleFile(t, filepath.Join(root, "sce_module", "readme.txt"), []byte("not a module"))
	info := &game.Info{Resources: root, ModuleDirs: []string{filepath.Join(root, "sce_module")}}
	staged, extra, _, err := stageModules(info, root, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("stageModules: %v", err)
	}
	if staged != "" || len(extra) != 0 {
		t.Fatalf("staged = %q, extra = %#v", staged, extra)
	}
}

func writeModule(t *testing.T, path string) {
	t.Helper()
	writeModuleFile(t, path, []byte("\x7fELF\x02\x01\x01\x00"))
}

func writeModuleFile(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}
