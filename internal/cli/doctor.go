// SPDX-License-Identifier: GPL-2.0-only

package cli

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"anygame/internal/anyps5"
)

type doctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

func runDoctor(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	flags.SetOutput(stderr)
	jsonOut := flags.Bool("json", false, "print JSON")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	var checks []doctorCheck
	add := func(name, status, detail string) {
		checks = append(checks, doctorCheck{Name: name, Status: status, Detail: detail})
	}

	if runtime.GOOS == "linux" || runtime.GOOS == "windows" {
		add("platform", "ok", fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH))
	} else {
		add("platform", "fail", fmt.Sprintf("%s is not a supported host", runtime.GOOS))
	}
	if runtime.GOARCH == "amd64" {
		add("architecture", "ok", "x86-64 host, the architecture PS5 titles are recompiled for")
	} else {
		add("architecture", "fail", fmt.Sprintf("%s cannot run recompiled PS5 titles", runtime.GOARCH))
	}

	if rt, err := anyps5.Ensure(runtime.GOOS, ""); err == nil {
		add("anyps5", "ok", fmt.Sprintf("AnyPS5 %s installed at %s", rt.Version, rt.Dir))
	} else {
		add("anyps5", "warn", "no AnyPS5 runtime for this host; run `anygame update`")
	}

	if runtime.GOOS == "windows" {
		add("cpu", "warn", "CPU extension check is not implemented on Windows; AnyPS5 validates the host at launch")
	}
	if runtime.GOOS == "linux" {
		flags := cpuFlags("/proc/cpuinfo")
		required := []string{"avx2", "fma", "f16c", "bmi1", "bmi2", "lzcnt", "movbe"}
		var missing []string
		for _, name := range required {
			if name == "lzcnt" && flags["abm"] {
				continue
			}
			if !flags[name] {
				missing = append(missing, name)
			}
		}
		if len(missing) == 0 {
			add("cpu", "ok", "all CPU extensions AnyPS5 requires are present")
		} else {
			add("cpu", "fail", "missing CPU extensions: "+strings.Join(missing, ", "))
		}
	}

	if path := vulkanLoader(); path != "" {
		add("vulkan", "ok", path)
	} else {
		add("vulkan", "warn", "no Vulkan loader found; install your GPU vendor's Vulkan driver")
	}
	if runtime.GOOS == "linux" {
		if info, err := os.Stat("/dev/udmabuf"); err == nil {
			file, openErr := os.OpenFile("/dev/udmabuf", os.O_RDWR, 0)
			if openErr == nil {
				file.Close()
				add("udmabuf", "ok", "/dev/udmabuf is readable and writable")
			} else {
				add("udmabuf", "warn", fmt.Sprintf("/dev/udmabuf exists with mode %s but is not writable: add the user to the kvm group", info.Mode().Perm()))
			}
		} else {
			add("udmabuf", "warn", "/dev/udmabuf is absent; shared direct memory falls back to copies")
		}
	}

	if *jsonOut {
		writeJSON(stdout, checks)
	} else {
		for _, check := range checks {
			fmt.Fprintf(stdout, "%-6s %-14s %s\n", strings.ToUpper(check.Status), check.Name, check.Detail)
		}
	}
	for _, check := range checks {
		if check.Status == "fail" {
			return 1
		}
	}
	return 0
}

func cpuFlags(path string) map[string]bool {
	flags := map[string]bool{}
	file, err := os.Open(path)
	if err != nil {
		return flags
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "flags") {
			continue
		}
		_, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		for _, flag := range strings.Fields(value) {
			flags[flag] = true
		}
		break
	}
	return flags
}

func vulkanLoader() string {
	if runtime.GOOS == "windows" {
		root := os.Getenv("SystemRoot")
		if root == "" {
			root = `C:\Windows`
		}
		candidates := []string{filepath.Join(root, "System32", "vulkan-1.dll")}
		for _, directory := range filepath.SplitList(os.Getenv("PATH")) {
			if directory != "" {
				candidates = append(candidates, filepath.Join(directory, "vulkan-1.dll"))
			}
		}
		for _, candidate := range candidates {
			if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() {
				return candidate
			}
		}
		return ""
	}
	candidates := []string{"/usr/lib/libvulkan.so.1", "/usr/lib64/libvulkan.so.1", "/usr/lib/x86_64-linux-gnu/libvulkan.so.1"}
	for _, candidate := range candidates {
		if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() {
			return candidate
		}
	}
	if path, err := exec.LookPath("vulkaninfo"); err == nil {
		return path
	}
	return ""
}
