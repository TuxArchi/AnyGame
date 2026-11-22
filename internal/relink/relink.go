// SPDX-License-Identifier: GPL-2.0-only

// Package relink runs the AnyPS5 relinker.
package relink

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Options configures one conversion.
type Options struct {
	Relinker      string
	Input         string
	Output        string
	Target        string
	SceModulePath string
	Registry      bool
	DryRun        bool
	Patches       string
	NoPatches     bool
	ModuleDirs    []string
	Executables   []string
	SkipModules   bool
	WindowsGUI    bool
	ToIntel       bool
	FrameRedZone  bool
	RPath         string
	Extra         []string
}

// Result describes a finished conversion.
type Result struct {
	Executable  string
	Needed      []string
	NeededKnown bool
	LogPath     string
}

// Args builds the relinker command line without the program name.
func (o *Options) Args() []string {
	var args []string
	if o.Target == "windows" {
		args = append(args, "--windows")
		if o.WindowsGUI {
			args = append(args, "--windows-gui")
		}
	}
	if o.ToIntel {
		args = append(args, "--to-intel")
	}
	if o.FrameRedZone && o.Target == "windows" {
		args = append(args, "--frame-red-zone")
	}
	if o.Registry {
		args = append(args, "--registry")
	}
	if o.DryRun {
		args = append(args, "--dry-run")
	}
	rpath := o.RPath
	if rpath == "" {
		rpath = "$ORIGIN/libs"
	}
	args = append(args, "--rpath", rpath)
	if o.SkipModules {
		args = append(args, "--skip-sce-module")
	} else if o.SceModulePath != "" {
		args = append(args, "--sce-module-path", o.SceModulePath)
	}
	for _, directory := range o.ModuleDirs {
		args = append(args, "--module-dir", directory)
	}
	for _, executable := range o.Executables {
		args = append(args, "--executable", executable)
	}
	if o.NoPatches {
		args = append(args, "--no-patches")
	} else if o.Patches != "" {
		args = append(args, "--patches", o.Patches)
	}
	args = append(args, o.Extra...)
	args = append(args, o.Input, o.Output)
	return args
}

// Run converts the input and returns the conversion result.
func Run(ctx context.Context, options Options, log io.Writer) (*Result, error) {
	if options.Relinker == "" {
		return nil, errors.New("no relinker binary configured")
	}
	if _, err := os.Stat(options.Relinker); err != nil {
		return nil, fmt.Errorf("relinker %s: %w", options.Relinker, err)
	}
	if err := os.MkdirAll(filepath.Dir(options.Output), 0o755); err != nil {
		return nil, err
	}
	arguments := options.Args()

	logPath := options.Output + ".conversion.log"
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}
	defer logFile.Close()

	writers := []io.Writer{logFile}
	if log != nil {
		writers = append(writers, log)
	}
	sink := io.MultiWriter(writers...)

	fmt.Fprintf(sink, "relinker %s\n", strings.Join(arguments, " "))
	command := exec.CommandContext(ctx, options.Relinker, arguments...)
	command.Stdout = sink
	command.Stderr = sink
	command.Dir = filepath.Dir(options.Output)
	runErr := command.Run()

	result := &Result{Executable: options.Output, LogPath: logPath}
	stem := strings.TrimSuffix(options.Output, filepath.Ext(options.Output))
	if needed, err := readNeeded(stem + ".needed.json"); err == nil {
		result.Needed = needed
		result.NeededKnown = true
	}
	if runErr != nil {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		return result, fmt.Errorf("relinker failed: %w\n%s", runErr, tail(logPath, 30))
	}
	if _, err := os.Stat(options.Output); err != nil {
		return result, fmt.Errorf("relinker reported success but %s does not exist\n%s", options.Output, tail(logPath, 30))
	}
	return result, nil
}

func readNeeded(path string) ([]string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var needed []string
	if err := json.Unmarshal(body, &needed); err != nil {
		return nil, err
	}
	return needed, nil
}

func tail(path string, lines int) string {
	body, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n")
	if len(parts) > lines {
		parts = parts[len(parts)-lines:]
	}
	return strings.Join(parts, "\n")
}
