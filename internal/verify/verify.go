// SPDX-License-Identifier: GPL-2.0-only

// Package verify checks that a converted package is internally consistent.
package verify

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"anygame/internal/elfpatch"
)

type Status string

const (
	StatusOK   Status = "ok"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
)

// Check is one verified property.
type Check struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
	Detail string `json:"detail"`
}

// Report is the result of checking a package.
type Report struct {
	Package     string   `json:"package"`
	Target      string   `json:"target"`
	Executable  string   `json:"executable"`
	OK          bool     `json:"ok"`
	Checks      []Check  `json:"checks"`
	MissingLibs []string `json:"missingLibraries,omitempty"`
	Warnings    int      `json:"warnings"`
	Errors      int      `json:"errors"`
}

type manifest struct {
	Format     string `json:"format"`
	Target     string `json:"target"`
	Executable string `json:"executable"`
}

// Run verifies the package in dir.
func Run(dir string) (*Report, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	body, err := os.ReadFile(filepath.Join(absolute, "anygame.json"))
	if err != nil {
		return nil, fmt.Errorf("%s is not an AnyGame package: %w", absolute, err)
	}
	var info manifest
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("anygame.json: %w", err)
	}
	if info.Executable == "" {
		return nil, errors.New("anygame.json has no executable")
	}
	report := &Report{Package: absolute, Target: info.Target}
	report.add("manifest", StatusOK, "anygame.json is valid")

	executable := filepath.Join(absolute, info.Executable)
	report.Executable = executable
	report.checkExecutable(executable, info.Target)
	report.checkLauncher(absolute, info.Target)

	libsDir := filepath.Join(absolute, "libs")
	libs, libsErr := directoryNames(libsDir)
	if libsErr != nil {
		report.add("libs", StatusFail, "libs/ is missing")
		libs = map[string]bool{}
	} else {
		report.add("libs", StatusOK, fmt.Sprintf("%d files in libs/", len(libs)))
	}

	bundled := app0Names(absolute)
	app0Eboot := filepath.Join(absolute, "app0", "eboot.bin")
	if _, err := os.Stat(app0Eboot); err != nil {
		report.add("resources", StatusWarn, "app0/eboot.bin is missing; some titles need it to locate their resources")
	} else {
		report.add("resources", StatusOK, "app0/eboot.bin present")
	}

	if info.Target == "windows" {
		report.checkWindowsDependencies(absolute, executable, libs, bundled)
	} else {
		report.checkLinuxDependencies(executable, libsDir, libs, bundled)
	}

	report.OK = report.Errors == 0
	return report, nil
}

func (r *Report) add(name string, status Status, detail string) {
	r.Checks = append(r.Checks, Check{Name: name, Status: status, Detail: detail})
	if status == StatusFail {
		r.Errors++
	}
	if status == StatusWarn {
		r.Warnings++
	}
}

func (r *Report) checkExecutable(path, target string) {
	info, err := os.Stat(path)
	if err != nil {
		r.add("executable", StatusFail, "executable is missing")
		return
	}
	if info.IsDir() {
		r.add("executable", StatusFail, "executable is a directory")
		return
	}
	header := make([]byte, 4)
	file, err := os.Open(path)
	if err == nil {
		_, _ = file.Read(header)
		file.Close()
	}
	switch {
	case bytes.Equal(header, []byte{0x7f, 'E', 'L', 'F'}):
		if target == "windows" {
			r.add("format", StatusFail, "package targets Windows but the executable is ELF")
		} else {
			r.add("format", StatusOK, "ELF executable")
		}
	case len(header) >= 2 && header[0] == 'M' && header[1] == 'Z':
		if target == "linux" {
			r.add("format", StatusFail, "package targets Linux but the executable is PE")
		} else {
			r.add("format", StatusOK, "PE executable")
		}
	default:
		r.add("format", StatusFail, "executable is neither ELF nor PE")
	}
	if target != "windows" && info.Mode().Perm()&0o111 == 0 {
		r.add("permissions", StatusFail, "executable is not executable; run chmod +x")
	}
}

func (r *Report) checkLauncher(dir, target string) {
	if target == "windows" {
		path := filepath.Join(dir, "run.bat")
		if _, err := os.Stat(path); err != nil {
			r.add("launcher", StatusWarn, "run.bat is missing")
			return
		}
		r.add("launcher", StatusOK, "run.bat present")
		return
	}
	path := filepath.Join(dir, "run.sh")
	info, err := os.Stat(path)
	if err != nil {
		r.add("launcher", StatusWarn, "run.sh is missing")
		return
	}
	if info.Mode().Perm()&0o111 == 0 {
		r.add("launcher", StatusWarn, "run.sh is not executable")
		return
	}
	r.add("launcher", StatusOK, "run.sh present")
}

func (r *Report) checkLinuxDependencies(executable, libsDir string, libs, bundled map[string]bool) {
	visited := map[string]bool{}
	r.checkELF(executable, libsDir, libs, bundled, visited, "executable")
	app0 := filepath.Join(filepath.Dir(executable), "app0")
	_ = filepath.WalkDir(app0, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		name := strings.ToLower(entry.Name())
		if strings.HasSuffix(name, ".guest.prx") {
			r.checkELF(path, libsDir, libs, bundled, visited, "module "+entry.Name())
		}
		return nil
	})
}

func (r *Report) checkELF(path, libsDir string, libs, bundled map[string]bool, visited map[string]bool, role string) {
	absolute, err := filepath.Abs(path)
	if err != nil || visited[absolute] {
		return
	}
	visited[absolute] = true
	needed, err := elfpatch.Needed(path)
	if err != nil {
		if errors.Is(err, elfpatch.ErrNotELF) {
			r.add(role, StatusWarn, "not an ELF image; dependencies not inspected")
			return
		}
		r.add(role, StatusWarn, err.Error())
		return
	}
	missing := 0
	for _, name := range needed {
		if name == "" {
			continue
		}
		if strings.Contains(name, "$ORIGIN") {
			resolved := strings.ReplaceAll(name, "$ORIGIN", filepath.Dir(path))
			if _, err := os.Stat(filepath.Clean(resolved)); err != nil {
				r.add(role, StatusFail, "bundled dependency is missing: "+name)
				missing++
			}
			continue
		}
		if bundled[name] || libs[name] || systemELFLibrary(name) {
			continue
		}
		if _, err := os.Stat(filepath.Join(libsDir, name)); err == nil {
			continue
		}
		r.add(role, StatusFail, "host dependency is missing from libs/: "+name)
		r.MissingLibs = appendUnique(r.MissingLibs, name)
		missing++
	}
	if missing == 0 {
		r.add(role, StatusOK, fmt.Sprintf("%d dependencies resolved", len(needed)))
	}
}

func (r *Report) checkNeededFile(path string, libs, bundled map[string]bool, strict bool) {
	body, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var names []string
	if err := json.Unmarshal(body, &names); err != nil {
		r.add("needed.json", StatusWarn, err.Error())
		return
	}
	for _, name := range names {
		if bundled[name] || libs[name] || systemELFLibrary(name) {
			continue
		}
		status := StatusWarn
		if strict {
			status = StatusFail
		}
		r.add("needed.json", status, "needed library is not present: "+name)
	}
}

func (r *Report) checkWindowsDependencies(dir, executable string, libs, bundled map[string]bool) {
	r.add("imports", StatusWarn, "PE import tables are not inspected yet; checking needed.json")
	stem := strings.TrimSuffix(executable, filepath.Ext(executable))
	for _, candidate := range []string{
		stem + ".needed.json",
		filepath.Join(dir, "anygame.needed.json"),
	} {
		if _, err := os.Stat(candidate); err == nil {
			body, err := os.ReadFile(candidate)
			if err != nil {
				continue
			}
			var names []string
			if err := json.Unmarshal(body, &names); err != nil {
				r.add("needed.json", StatusWarn, err.Error())
				continue
			}
			for _, name := range names {
				if bundled[name] || libs[name] || systemDLL(name) {
					continue
				}
				r.add("needed.json", StatusFail, "DLL is missing from libs/: "+name)
				r.MissingLibs = appendUnique(r.MissingLibs, name)
			}
		}
	}
	for _, name := range []string{"libgcc_s_seh-1.dll", "libstdc++-6.dll", "libwinpthread-1.dll"} {
		if !libs[name] {
			r.add("toolchain", StatusWarn, name+" is missing from libs/")
		}
	}
}

func app0Names(dir string) map[string]bool {
	result := map[string]bool{}
	_ = filepath.WalkDir(filepath.Join(dir, "app0"), func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		name := entry.Name()
		result[name] = true
		if strings.HasSuffix(name, ".guest.prx") {
			result[strings.TrimSuffix(name, ".guest.prx")] = true
		}
		return nil
	})
	return result
}

func directoryNames(dir string) (map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	result := map[string]bool{}
	for _, entry := range entries {
		if !entry.IsDir() {
			result[entry.Name()] = true
		}
	}
	return result, nil
}

func systemELFLibrary(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, ".so") || strings.HasPrefix(lower, "ld-linux")
}

func systemDLL(name string) bool {
	upper := strings.ToUpper(filepath.Base(name))
	if strings.HasPrefix(upper, "API-MS-WIN-") || strings.HasPrefix(upper, "EXT-MS-") {
		return true
	}
	switch upper {
	case "KERNEL32.DLL", "USER32.DLL", "GDI32.DLL", "ADVAPI32.DLL", "SHELL32.DLL",
		"OLE32.DLL", "COMDLG32.DLL", "WS2_32.DLL", "WINMM.DLL", "IMM32.DLL",
		"VERSION.DLL", "SETUPAPI.DLL", "DWMAPI.DLL", "UXTHEME.DLL", "BCRYPT.DLL",
		"CRYPT32.DLL", "SECUR32.DLL", "IPHLPAPI.DLL", "DNSAPI.DLL", "MSIMG32.DLL",
		"GDIPLUS.DLL", "OPENGL32.DLL", "VULKAN-1.DLL", "WININET.DLL", "WLDAP32.DLL":
		return true
	}
	return false
}

func appendUnique(values []string, value string) []string {
	for _, item := range values {
		if item == value {
			return values
		}
	}
	return append(values, value)
}
