// SPDX-License-Identifier: GPL-2.0-only

package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"anygame/internal/game"
	"anygame/internal/self"
)

// stageModules copies or unwraps the guest modules of a title into a clean
// staging tree, because the relinker refuses SELF containers and reads the
// module directories beside its input.
func stageModules(info *game.Info, moduleParent, workDir string, userDirs []string) (string, []string, []string, error) {
	type moduleDir struct {
		relative string
		absolute string
		extra    bool
	}
	seen := map[string]bool{}
	var dirs []moduleDir
	add := func(absolute string, extra bool) error {
		relative, err := filepath.Rel(moduleParent, absolute)
		if err != nil || strings.HasPrefix(relative, "..") {
			return fmt.Errorf("%s is not under the module parent %s", absolute, moduleParent)
		}
		relative = filepath.ToSlash(filepath.Clean(relative))
		if relative == "." || seen[relative] {
			return nil
		}
		seen[relative] = true
		dirs = append(dirs, moduleDir{relative: relative, absolute: absolute, extra: extra})
		return nil
	}
	for _, directory := range info.ModuleDirs {
		if err := add(directory, false); err != nil {
			return "", nil, nil, err
		}
	}
	for _, relative := range info.ExtraModuleDirs {
		if err := add(filepath.Join(info.Resources, filepath.FromSlash(relative)), true); err != nil {
			return "", nil, nil, err
		}
	}
	for _, directory := range userDirs {
		absolute := directory
		if !filepath.IsAbs(absolute) {
			absolute = filepath.Join(moduleParent, absolute)
		}
		if err := add(absolute, true); err != nil {
			return "", nil, nil, err
		}
	}
	sort.SliceStable(dirs, func(i, j int) bool { return dirs[i].relative < dirs[j].relative })

	stagingRoot := filepath.Join(workDir, "modules")
	var extra []string
	var warnings []string
	for _, directory := range dirs {
		entries, err := os.ReadDir(directory.absolute)
		if err != nil {
			return "", nil, nil, err
		}
		staged := 0
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			lower := strings.ToLower(name)
			if !strings.HasSuffix(lower, ".prx") && !strings.HasSuffix(lower, ".sprx") {
				continue
			}
			source := filepath.Join(directory.absolute, name)
			destination := filepath.Join(stagingRoot, filepath.FromSlash(directory.relative), name)
			if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
				return "", nil, nil, err
			}
			kind, err := moduleKind(source)
			if err != nil {
				return "", nil, nil, err
			}
			switch kind {
			case "self":
				if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
					return "", nil, nil, err
				}
				if _, err := self.UnwrapFile(source, destination, nil); err != nil {
					return "", nil, nil, fmt.Errorf("%s: %w", source, err)
				}
				staged++
			case "elf":
				if err := copyModule(source, destination); err != nil {
					return "", nil, nil, err
				}
				staged++
			default:
				warnings = append(warnings, fmt.Sprintf("module %s is neither ELF nor SELF; skipped", source))
			}
		}
		if staged == 0 {
			continue
		}
		if directory.extra {
			extra = append(extra, directory.relative)
		}
	}
	if len(extra) == 0 && stagingRootIsEmpty(stagingRoot) {
		return "", nil, warnings, nil
	}
	return stagingRoot, extra, warnings, nil
}

func moduleKind(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	header := make([]byte, 4)
	if _, err := io.ReadFull(file, header); err != nil {
		return "other", nil
	}
	if bytes.Equal(header, []byte{0x7f, 'E', 'L', 'F'}) {
		return "elf", nil
	}
	if bytes.Equal(header, []byte{0x4f, 0x15, 0x3d, 0x1d}) || bytes.Equal(header, []byte{0x54, 0x14, 0xf5, 0xee}) {
		return "self", nil
	}
	return "other", nil
}

func copyModule(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func moduleExclusions(info *game.Info) []string {
	seen := map[string]bool{}
	var result []string
	add := func(relative string) {
		relative = filepath.ToSlash(filepath.Clean(relative))
		if relative == "." || relative == "" || seen[relative] {
			return
		}
		seen[relative] = true
		result = append(result, relative)
	}
	for _, directory := range info.ModuleDirs {
		if relative, err := filepath.Rel(info.Resources, directory); err == nil && !strings.HasPrefix(relative, "..") {
			add(relative)
		}
	}
	for _, relative := range info.ExtraModuleDirs {
		add(relative)
	}
	return result
}

func stagingRootIsEmpty(path string) bool {
	entries, err := os.ReadDir(path)
	return err != nil || len(entries) == 0
}
