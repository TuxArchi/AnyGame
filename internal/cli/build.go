// SPDX-License-Identifier: GPL-2.0-only

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"anygame/internal/anyps5"
	"anygame/internal/game"
	"anygame/internal/packaging"
	"anygame/internal/relink"
	"anygame/internal/self"
)

func runBuild(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	flags.SetOutput(stderr)
	output := flags.String("o", "", "output directory (default: ./<title id>)")
	outputLong := flags.String("output", "", "output directory (alias of -o)")
	targetFlag := flags.String("target", runtime.GOOS, "target platform: linux or windows")
	version := flags.String("version", "", "AnyPS5 runtime version (default: active)")
	name := flags.String("name", "", "executable base name (default: title id)")
	patches := flags.String("patches", "", "game patch TOML to apply")
	noPatches := flags.Bool("no-patches", false, "do not apply or discover game patches")
	resources := flags.String("resources", "link", "how to place app0 resources: link, copy or none")
	skipModules := flags.Bool("skip-modules", false, "convert without bundled guest modules")
	windowsGUI := flags.Bool("windows-gui", false, "Windows: build a GUI-subsystem executable")
	toIntel := flags.Bool("to-intel", false, "convert AMD-only instructions for Intel CPUs")
	keepWork := flags.Bool("keep-work", false, "keep the conversion work directory")
	dryRun := flags.Bool("dry-run", false, "plan the conversion without writing the package")
	jsonOut := flags.Bool("json", false, "print JSON")
	var moduleDirs, executables StringList
	flags.Var(&moduleDirs, "module-dir", "additional module directory (repeatable)")
	flags.Var(&executables, "executable", "additional title executable (repeatable)")
	positionals, err := parsePositionals(flags, args,
		"no-patches", "skip-modules", "windows-gui", "to-intel", "keep-work", "dry-run", "json")
	if err != nil {
		return 2
	}
	if len(positionals) < 1 {
		return fail(stderr, fmt.Errorf("missing title path"))
	}
	input := positionals[0]
	if len(positionals) > 1 {
		return fail(stderr, fmt.Errorf("build takes one title path"))
	}
	target, err := anyps5.NormalizeTarget(*targetFlag)
	if err != nil {
		return fail(stderr, err)
	}
	if target != "linux" && target != "windows" {
		return fail(stderr, fmt.Errorf("unsupported host %s: AnyGame needs Linux or Windows", target))
	}
	mode := packaging.ResourcesMode(*resources)
	switch mode {
	case packaging.ResourcesLink, packaging.ResourcesCopy, packaging.ResourcesNone:
	default:
		return fail(stderr, fmt.Errorf("invalid --resources %q: use link, copy or none", *resources))
	}

	info, err := game.Discover(input)
	if err != nil {
		return fail(stderr, err)
	}
	rt, err := anyps5.Ensure(target, *version)
	if err != nil {
		return fail(stderr, err)
	}
	outputDir := *output
	if outputDir == "" {
		outputDir = *outputLong
	}
	working, _ := os.Getwd()
	if outputDir == "" {
		outputDir = defaultOutputDir(info)
	} else if !filepath.IsAbs(outputDir) {
		outputDir = filepath.Join(working, outputDir)
	}
	outputDir, err = filepath.Abs(outputDir)
	if err != nil {
		return fail(stderr, err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fail(stderr, err)
	}

	baseName := resolveTitleName(info)
	if *name != "" {
		baseName = sanitizeFilename(*name)
	}
	baseName = strings.TrimSuffix(baseName, filepath.Ext(baseName))
	extension := ".elf"
	if target == "windows" {
		extension = ".exe"
	}
	executablePath := filepath.Join(outputDir, baseName+extension)

	if !*skipModules && len(info.ModuleDirs) == 0 && len(moduleDirs) == 0 {
		fmt.Fprintf(stderr, "warning: %s has no sce_module/sce_modules/prx directory; pass --skip-modules for a title without guest modules\n", info.Resources)
	}
	if len(info.ModuleDirs) > 1 {
		fmt.Fprintf(stderr, "warning: multiple module directories found: %s; the relinker accepts one\n", strings.Join(info.ModuleDirs, ", "))
	}

	workDir := filepath.Join(outputDir, ".anygame-work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return fail(stderr, err)
	}
	removeWork := func() {
		if !*keepWork {
			os.RemoveAll(workDir)
		}
	}
	defer removeWork()

	conversionInput := info.Eboot
	if info.Kind == game.KindSELF {
		conversionInput = filepath.Join(workDir, "eboot.elf")
		fmt.Fprintf(stderr, "Unwrapping SELF container %s...\n", filepath.Base(info.Eboot))
		if _, err := self.UnwrapFile(info.Eboot, conversionInput, nil); err != nil {
			return fail(stderr, fmt.Errorf("unwrap %s: %w", info.Eboot, err))
		}
	}

	patchesPath := *patches
	if patchesPath == "" && !*noPatches {
		for _, candidate := range []string{
			filepath.Join(info.Resources, "relinker-patches.toml"),
			filepath.Join(info.Root, "relinker-patches.toml"),
		} {
			if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() {
				patchesPath = candidate
				fmt.Fprintf(stderr, "Using game patches from %s\n", candidate)
				break
			}
		}
	}
	options := relink.Options{
		Relinker:      rt.Relinker,
		Input:         conversionInput,
		Output:        executablePath,
		Target:        target,
		SceModulePath: info.Resources,
		Registry:      true,
		Patches:       patchesPath,
		NoPatches:     *noPatches,
		ModuleDirs:    moduleDirs,
		Executables:   executables,
		SkipModules:   *skipModules,
		WindowsGUI:    *windowsGUI,
		ToIntel:       *toIntel,
		RPath:         "$ORIGIN/libs",
		DryRun:        *dryRun,
	}
	if *skipModules {
		options.SceModulePath = ""
	}
	fmt.Fprintf(stderr, "Converting %s for %s with AnyPS5 %s...\n", filepath.Base(info.Eboot), target, rt.Version)
	result, err := relink.Run(ctx, options, os.Stderr)
	if err != nil {
		if *keepWork {
			fmt.Fprintf(stderr, "work directory kept at %s\n", workDir)
		}
		return fail(stderr, err)
	}
	if *dryRun {
		fmt.Fprintf(stdout, "Dry run complete. Would write %s and package %s resources.\n", executablePath, mode)
		return 0
	}

	packageResult, err := packaging.Package(packaging.Options{
		OutputDir:   outputDir,
		Executable:  executablePath,
		Name:        baseName,
		Target:      target,
		Runtime:     rt,
		Game:        info,
		Needed:      result.Needed,
		NeededKnown: result.NeededKnown,
		Resources:   mode,
	})
	if err != nil {
		return fail(stderr, err)
	}

	type buildReport struct {
		Title      string   `json:"title"`
		TitleID    string   `json:"titleId"`
		Target     string   `json:"target"`
		Runtime    string   `json:"runtime"`
		Executable string   `json:"executable"`
		Directory  string   `json:"directory"`
		Launch     string   `json:"launch"`
		Libraries  []string `json:"libraries"`
		Warnings   []string `json:"warnings,omitempty"`
	}
	report := buildReport{
		Title: info.TitleName, TitleID: info.TitleID, Target: target, Runtime: rt.Version,
		Executable: packageResult.Executable, Directory: outputDir,
		Launch: "anygame run " + quotePath(outputDir), Libraries: packageResult.Libraries,
		Warnings: append(info.Warnings, packageResult.Warnings...),
	}
	if *jsonOut {
		writeJSON(stdout, report)
		return 0
	}
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "Title:      %s (%s)\n", valueOr(info.TitleName, "unknown"), valueOr(info.TitleID, "no id"))
	fmt.Fprintf(stdout, "Target:     %s with AnyPS5 %s\n", target, rt.Version)
	fmt.Fprintf(stdout, "Executable: %s\n", executablePath)
	fmt.Fprintf(stdout, "Libraries:  %d\n", len(packageResult.Libraries))
	fmt.Fprintf(stdout, "Package:    %s\n", outputDir)
	for _, warning := range report.Warnings {
		fmt.Fprintf(stdout, "warning:    %s\n", warning)
	}
	fmt.Fprintf(stdout, "\nLaunch it with: anygame run %s\n", quotePath(outputDir))
	return 0
}

func quotePath(path string) string {
	if strings.ContainsAny(path, " \t\"'") {
		return "\"" + path + "\""
	}
	return path
}

type packageManifest struct {
	Executable string `json:"executable"`
	Title      struct {
		Name string `json:"name"`
	} `json:"title"`
}

func runGame(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	var before, extra []string
	for index, argument := range args {
		if argument == "--" {
			before = args[:index]
			extra = args[index+1:]
			break
		}
	}
	if before == nil {
		before = args
	}
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.SetOutput(stderr)
	positionals, err := parsePositionals(flags, before)
	if err != nil {
		return 2
	}
	if len(positionals) < 1 {
		return fail(stderr, errors.New("usage: anygame run <package directory> [-- arguments...]"))
	}
	path := positionals[0]
	extra = append(positionals[1:], extra...)
	if path == "" {
		return fail(stderr, errors.New("usage: anygame run <package directory> [-- arguments...]"))
	}
	directory, err := filepath.Abs(path)
	if err != nil {
		return fail(stderr, err)
	}
	executable, err := findExecutable(directory)
	if err != nil {
		return fail(stderr, err)
	}
	command := exec.CommandContext(ctx, executable, extra...)
	command.Dir = directory
	command.Stdin = os.Stdin
	command.Stdout = stdout
	command.Stderr = stderr
	command.Env = os.Environ()
	if err := command.Run(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			fmt.Fprintf(stderr, "game exited with status %d\n", exitError.ExitCode())
			return exitError.ExitCode()
		}
		return fail(stderr, err)
	}
	return 0
}

func findExecutable(directory string) (string, error) {
	manifestPath := filepath.Join(directory, "anygame.json")
	if body, err := os.ReadFile(manifestPath); err == nil {
		var manifest packageManifest
		if json.Unmarshal(body, &manifest) == nil && manifest.Executable != "" {
			candidate := filepath.Join(directory, manifest.Executable)
			if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() {
				return candidate, nil
			}
		}
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if strings.HasSuffix(name, ".elf") || strings.HasSuffix(name, ".exe") {
			return filepath.Join(directory, entry.Name()), nil
		}
	}
	return "", fmt.Errorf("no converted executable in %s (run `anygame build` first)", directory)
}
