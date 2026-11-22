// SPDX-License-Identifier: GPL-2.0-only

// Package diagnose launches a converted package for a bounded time and reads
// the runtime log for missing dependencies, missing resources and fatal errors.
package diagnose

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// Findings is the analysis of one diagnostic run.
type Findings struct {
	Verdict          string         `json:"verdict"`
	Started          bool           `json:"started"`
	Window           bool           `json:"window"`
	Exited           bool           `json:"exited"`
	TimedOut         bool           `json:"timedOut"`
	ExitCode         int            `json:"exitCode"`
	Seconds          int            `json:"seconds"`
	LogPath          string         `json:"logPath"`
	MissingLibraries []string       `json:"missingLibraries,omitempty"`
	MissingFiles     []string       `json:"missingFiles,omitempty"`
	NotImplemented   []string       `json:"notImplemented,omitempty"`
	Fatal            []string       `json:"fatal,omitempty"`
	Warnings         []string       `json:"warnings,omitempty"`
	Tail             []string       `json:"tail,omitempty"`
	Report           map[string]any `json:"report,omitempty"`
}

type manifest struct {
	Target     string `json:"target"`
	Executable string `json:"executable"`
}

var (
	sharedLibraryPattern = regexp.MustCompile(`error while loading shared libraries: ([^:]+):`)
	dlopenPattern        = regexp.MustCompile(`dlopen: cannot load .*? \(([^:]+): cannot open shared object file`)
	openFailedPattern    = regexp.MustCompile(`open failed path=(\S+)`)
)

// Run starts the package executable for seconds, then kills it and analyzes
// the captured log. The package manifest is updated with a diagnosis summary.
func Run(dir string, seconds int) (*Findings, error) {
	if seconds < 3 {
		seconds = 3
	}
	if seconds > 300 {
		seconds = 300
	}
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
		return nil, fmt.Errorf("anygame.json has no executable")
	}
	if info.Target != "" && info.Target != runtime.GOOS {
		return nil, fmt.Errorf("this package targets %s and cannot run on %s; use anygame verify instead", info.Target, runtime.GOOS)
	}
	executable := filepath.Join(absolute, info.Executable)
	if _, err := os.Stat(executable); err != nil {
		return nil, fmt.Errorf("executable: %w", err)
	}
	logPath := filepath.Join(absolute, "anygame-diagnose.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}
	_ = os.Remove(filepath.Join(absolute, "anyps5-report.json"))

	command := exec.Command(executable)
	command.Dir = absolute
	command.Env = environment(os.Environ(), map[string]string{
		"APS5_LOG_SYNC":     "1",
		"APS5_REPORT":       "1",
		"ANYPS5_FULLSCREEN": "0",
	})
	command.Stdout = logFile
	command.Stderr = logFile
	setProcessGroup(command)
	if err := command.Start(); err != nil {
		logFile.Close()
		return nil, fmt.Errorf("launch %s: %w", executable, err)
	}

	findings := &Findings{Seconds: seconds, LogPath: logPath}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case waitErr := <-done:
		findings.Exited = true
		findings.ExitCode = exitCode(waitErr)
	case <-time.After(time.Duration(seconds) * time.Second):
		findings.TimedOut = true
		killProcess(command)
		<-done
		findings.ExitCode = -1
	}
	logFile.Close()

	logBody, err := os.ReadFile(logPath)
	if err != nil {
		return nil, err
	}
	analyzed := Analyze(string(logBody))
	analyzed.Seconds = seconds
	analyzed.LogPath = logPath
	analyzed.Exited = findings.Exited
	analyzed.TimedOut = findings.TimedOut
	analyzed.ExitCode = findings.ExitCode
	analyzed.Report = readReport(filepath.Join(absolute, "anyps5-report.json"))
	analyzed.finalize()
	findings = analyzed
	updateManifest(absolute, findings)
	return findings, nil
}

// Analyze reads a runtime log and extracts the findings.
func Analyze(log string) *Findings {
	findings := &Findings{}
	scanner := bufio.NewScanner(strings.NewReader(log))
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
		switch {
		case strings.Contains(line, "Swapchain created"):
			findings.Window = true
			findings.Started = true
		case strings.Contains(line, "Vulkan device ready"):
			findings.Started = true
		case strings.Contains(line, "RunThread entry"):
			findings.Started = true
		}
		if match := sharedLibraryPattern.FindStringSubmatch(line); match != nil {
			findings.MissingLibraries = appendUnique(findings.MissingLibraries, filepath.Base(strings.TrimSpace(match[1])))
		}
		if match := dlopenPattern.FindStringSubmatch(line); match != nil {
			findings.MissingLibraries = appendUnique(findings.MissingLibraries, filepath.Base(strings.TrimSpace(match[1])))
		}
		if match := openFailedPattern.FindStringSubmatch(line); match != nil {
			findings.MissingFiles = appendUnique(findings.MissingFiles, strings.TrimSpace(match[1]))
		}
		lower := strings.ToLower(line)
		if !strings.Contains(line, "\":") &&
			(strings.Contains(lower, "notimplemented") || strings.Contains(lower, "not implemented") || strings.Contains(lower, "unimplemented")) {
			findings.NotImplemented = appendUnique(findings.NotImplemented, strings.TrimSpace(line))
		}
		switch {
		case strings.Contains(line, "FATAL"), strings.Contains(line, "unhandled exception"),
			strings.Contains(line, "Segmentation fault"), strings.Contains(line, "core dumped"),
			strings.Contains(line, "Aborted"), strings.Contains(line, "Illegal instruction"):
			findings.Fatal = appendUnique(findings.Fatal, strings.TrimSpace(line))
		}
		if strings.Contains(line, "IsNetworkReachable is false") {
			findings.Warnings = appendUnique(findings.Warnings, "the title reports no network connectivity")
		}
	}
	if len(lines) > 15 {
		lines = lines[len(lines)-15:]
	}
	findings.Tail = lines
	findings.finalize()
	return findings
}

func (f *Findings) finalize() {
	switch {
	case len(f.Fatal) > 0:
		f.Verdict = "crashed"
	case f.Window && f.TimedOut:
		f.Verdict = "running"
	case f.Window && f.Exited && f.ExitCode == 0:
		f.Verdict = "exited"
	case f.Window && f.Exited:
		f.Verdict = "crashed"
	case f.Window:
		f.Verdict = "running"
	case len(f.MissingLibraries) > 0:
		f.Verdict = "missing-libraries"
	case f.Started:
		f.Verdict = "started"
	default:
		f.Verdict = "no-window"
	}
}

func readReport(path string) map[string]any {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var report map[string]any
	if json.Unmarshal(body, &report) != nil {
		return nil
	}
	keys := []string{"stage", "framesPresented", "firstDraw", "firstPresent", "exit", "titleId"}
	selected := map[string]any{}
	for _, key := range keys {
		if value, ok := report[key]; ok {
			selected[key] = value
		}
	}
	if len(selected) == 0 {
		return nil
	}
	return selected
}

func updateManifest(dir string, findings *Findings) {
	path := filepath.Join(dir, "anygame.json")
	body, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var manifest map[string]any
	if json.Unmarshal(body, &manifest) != nil {
		return
	}
	manifest["diagnosis"] = map[string]any{
		"at":               time.Now().UTC().Format(time.RFC3339),
		"verdict":          findings.Verdict,
		"seconds":          findings.Seconds,
		"window":           findings.Window,
		"missingLibraries": findings.MissingLibraries,
		"log":              filepath.Base(findings.LogPath),
	}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return
	}
	encoded = append(encoded, '\n')
	temporary := path + ".tmp"
	if os.WriteFile(temporary, encoded, 0o644) == nil {
		_ = os.Rename(temporary, path)
	}
}

func environment(base []string, overrides map[string]string) []string {
	var result []string
	for _, entry := range base {
		name, _, found := strings.Cut(entry, "=")
		if found {
			if _, override := overrides[name]; override {
				continue
			}
		}
		result = append(result, entry)
	}
	for name, value := range overrides {
		result = append(result, name+"="+value)
	}
	return result
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitError, ok := err.(*exec.ExitError); ok {
		return exitError.ExitCode()
	}
	return -1
}

func appendUnique(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, item := range values {
		if item == value {
			return values
		}
	}
	return append(values, value)
}
