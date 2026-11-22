// SPDX-License-Identifier: GPL-2.0-only

// Package packaging assembles a converted title into AnyPS5's runtime layout.
package packaging

import (
	"debug/elf"
	"debug/pe"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"anygame/internal/anyps5"
	"anygame/internal/elfpatch"
	"anygame/internal/game"
)

// ResourcesMode selects how title resources enter app0/.
type ResourcesMode string

const (
	ResourcesLink ResourcesMode = "link"
	ResourcesCopy ResourcesMode = "copy"
	ResourcesNone ResourcesMode = "none"
)

// Options configures packaging.
type Options struct {
	OutputDir   string
	Executable  string
	Name        string
	Target      string
	Runtime     *anyps5.Runtime
	Game        *game.Info
	Needed      []string
	NeededKnown bool
	Resources   ResourcesMode
	ExcludeDirs []string
	Warnings    []string
}

// Manifest records how a package was produced.
type Manifest struct {
	Format      string      `json:"format"`
	Title       TitleInfo   `json:"title"`
	Target      string      `json:"target"`
	Executable  string      `json:"executable"`
	Runtime     RuntimeInfo `json:"runtime"`
	ConvertedAt time.Time   `json:"convertedAt"`
	Source      SourceInfo  `json:"source"`
	Libraries   []string    `json:"libraries"`
	Warnings    []string    `json:"warnings,omitempty"`
}

// TitleInfo is the guest metadata of the package.
type TitleInfo struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	ContentVersion string `json:"contentVersion,omitempty"`
	Language       string `json:"language,omitempty"`
}

// RuntimeInfo is the AnyPS5 build used for the conversion.
type RuntimeInfo struct {
	Version  string `json:"version"`
	Relinker string `json:"relinker"`
	Libs     string `json:"libs"`
}

// SourceInfo identifies the input executable.
type SourceInfo struct {
	Eboot  string `json:"eboot"`
	Bytes  int64  `json:"bytes"`
	Format string `json:"format"`
}

// Result describes the assembled package.
type Result struct {
	Dir          string
	Executable   string
	LaunchScript string
	Manifest     string
	Libraries    []string
	Warnings     []string
}

// Package builds the runtime layout around an already converted executable.
func Package(options Options) (*Result, error) {
	if options.OutputDir == "" {
		return nil, errors.New("empty output directory")
	}
	if options.Executable == "" {
		return nil, errors.New("empty executable")
	}
	if options.Runtime == nil {
		return nil, errors.New("no AnyPS5 runtime")
	}
	if options.Resources == "" {
		options.Resources = ResourcesLink
	}
	if err := os.MkdirAll(options.OutputDir, 0o755); err != nil {
		return nil, err
	}
	result := &Result{Dir: options.OutputDir, Executable: filepath.Base(options.Executable)}
	warnings := append([]string(nil), options.Warnings...)
	needed := options.Needed
	if options.Target == "linux" && options.NeededKnown {
		dropped, notes, pruneErr := pruneUnusedDependencies(options.Executable, options.OutputDir, options.Runtime.LibsDir, needed)
		if pruneErr != nil {
			warnings = append(warnings, fmt.Sprintf("could not prune unused dependencies: %v", pruneErr))
		} else if len(dropped) > 0 {
			needed = removeNames(needed, dropped)
			warnings = append(warnings, notes...)
		}
	}

	libraries, libraryWarnings, err := copyLibraries(options.Runtime.LibsDir, needed, options.NeededKnown, options.Target, options.OutputDir, options.Executable)
	if err != nil {
		return nil, err
	}
	result.Libraries = libraries
	if result.Libraries == nil {
		result.Libraries = []string{}
	}
	warnings = append(warnings, libraryWarnings...)

	if options.Resources != ResourcesNone && options.Game != nil {
		app0 := filepath.Join(options.OutputDir, "app0")
		if err := os.MkdirAll(app0, 0o755); err != nil {
			return nil, err
		}
		excluded := map[string]bool{}
		if filepath.Base(options.Game.Resources) != "app0" {
			excluded["app0"] = true
		}
		for _, directory := range options.ExcludeDirs {
			excluded[filepath.ToSlash(filepath.Clean(directory))] = true
		}
		if len(options.ExcludeDirs) == 0 {
			for _, directory := range options.Game.ModuleDirs {
				excluded[filepath.Base(directory)] = true
			}
		}
		excluded[filepath.Base(options.Game.Eboot)] = true
		if err := copyResources(options.Game.Resources, app0, options.OutputDir, options.Resources == ResourcesLink, excluded); err != nil {
			return nil, err
		}
		original := filepath.Join(app0, "eboot.bin")
		if filepath.Clean(options.Game.Eboot) != filepath.Clean(original) {
			if err := copyFile(options.Game.Eboot, original, 0o644); err != nil {
				return nil, fmt.Errorf("copy original eboot: %w", err)
			}
		}
	}

	launcher, err := writeLauncher(options)
	if err != nil {
		return nil, err
	}
	result.LaunchScript = launcher

	if options.Target == "linux" {
		entryPath, err := writeDesktopEntry(options)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("desktop entry: %v", err))
		} else {
			// The relinker writes its own desktop entry next to the executable;
			// the package keeps a single entry that goes through run.sh.
			relinkerEntry := filepath.Join(options.OutputDir, filepath.Base(options.Executable)+".desktop")
			if filepath.Clean(entryPath) != filepath.Clean(relinkerEntry) {
				_ = os.Remove(relinkerEntry)
			}
		}
	}

	manifest := Manifest{
		Format:      "anygame.package/1",
		Target:      options.Target,
		Executable:  filepath.Base(options.Executable),
		ConvertedAt: time.Now().UTC(),
		Libraries:   result.Libraries,
		Warnings:    warnings,
	}
	if options.Game != nil {
		manifest.Title = TitleInfo{
			ID: options.Game.TitleID, Name: options.Game.TitleName,
			ContentVersion: options.Game.ContentVersion, Language: options.Game.Language,
		}
		manifest.Source = SourceInfo{
			Eboot: filepath.Base(options.Game.Eboot), Bytes: options.Game.EbootSize, Format: string(options.Game.Kind),
		}
	}
	if options.Runtime != nil {
		manifest.Runtime = RuntimeInfo{
			Version: options.Runtime.Version, Relinker: options.Runtime.Manifest.Relinker.Name,
			Libs: options.Runtime.Manifest.Libs.Name,
		}
	}
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	body = append(body, '\n')
	manifestPath := filepath.Join(options.OutputDir, "anygame.json")
	if err := os.WriteFile(manifestPath, body, 0o644); err != nil {
		return nil, err
	}
	result.Manifest = manifestPath
	result.Warnings = warnings
	return result, nil
}

type registryEntry struct {
	Library string `json:"library"`
}

// pruneUnusedDependencies drops DT_NEEDED entries that the relinker declared
// but that no import uses and that no installed or bundled library provides.
// It patches the main executable and every converted module in app0.
func pruneUnusedDependencies(executable, outputDir, libsDir string, needed []string) ([]string, []string, error) {
	stem := strings.TrimSuffix(executable, filepath.Ext(executable))
	bundled := map[string]bool{}
	app0 := filepath.Join(outputDir, "app0")
	_ = filepath.WalkDir(app0, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return nil
		}
		bundled[entry.Name()] = true
		if strings.HasSuffix(entry.Name(), ".guest.prx") {
			bundled[strings.TrimSuffix(entry.Name(), ".guest.prx")] = true
		}
		return nil
	})

	var dropped, notes []string
	mainDropped, mainNotes, err := pruneOne(executable, stem+".registry.json", libsDir, bundled)
	if err != nil {
		return nil, nil, err
	}
	dropped = append(dropped, mainDropped...)
	notes = append(notes, mainNotes...)

	_ = filepath.WalkDir(app0, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		name := strings.ToLower(entry.Name())
		if !strings.HasSuffix(name, ".prx") && !strings.HasSuffix(name, ".sprx") {
			return nil
		}
		registry := filepath.Join(outputDir, filepath.Base(stem)+"."+entry.Name()+".registry.json")
		if _, statErr := os.Stat(registry); statErr != nil {
			return nil
		}
		_, moduleNotes, pruneErr := pruneOne(path, registry, libsDir, bundled)
		if pruneErr != nil {
			notes = append(notes, fmt.Sprintf("%s: %v", entry.Name(), pruneErr))
			return nil
		}
		notes = append(notes, moduleNotes...)
		return nil
	})
	return dropped, notes, nil
}

func pruneOne(file, registryPath, libsDir string, bundled map[string]bool) ([]string, []string, error) {
	body, err := os.ReadFile(registryPath)
	if err != nil {
		return nil, nil, nil
	}
	var entries []registryEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, nil, err
	}
	imports := map[string]bool{}
	for _, entry := range entries {
		if entry.Library != "" {
			imports[entry.Library] = true
		}
	}
	names, err := elfpatch.Needed(file)
	if err != nil {
		if errors.Is(err, elfpatch.ErrNotELF) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	candidates := map[string]bool{}
	for _, name := range names {
		if name == "" || imports[name] || bundled[name] || !isRuntimeLibrary(name) {
			continue
		}
		if strings.Contains(name, "$ORIGIN") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
			continue
		}
		if _, err := os.Stat(filepath.Join(libsDir, name)); err == nil {
			continue
		}
		candidates[name] = true
	}
	if len(candidates) == 0 {
		return nil, nil, nil
	}
	dropped, err := elfpatch.DropNeeded(file, candidates)
	if err != nil {
		return nil, nil, err
	}
	var notes []string
	for _, name := range dropped {
		notes = append(notes, fmt.Sprintf("ignored unused dependency %s in %s: declared but imported nowhere", name, filepath.Base(file)))
	}
	return dropped, notes, nil
}

func removeNames(values, remove []string) []string {
	drop := map[string]bool{}
	for _, name := range remove {
		drop[name] = true
	}
	var result []string
	for _, name := range values {
		if !drop[name] {
			result = append(result, name)
		}
	}
	return result
}

func copyLibraries(libsDir string, needed []string, neededKnown bool, platform, outputDir, executable string) ([]string, []string, error) {
	destinationDir := filepath.Join(outputDir, "libs")
	if err := os.MkdirAll(destinationDir, 0o755); err != nil {
		return nil, nil, err
	}
	seeds := append([]string(nil), needed...)
	seeds = append(seeds, bundledModuleDependencies(outputDir)...)
	seeds = append(seeds, moduleNeededDependencies(outputDir, executable)...)
	names, warnings, err := resolveClosure(libsDir, seeds, neededKnown)
	if err != nil {
		return nil, nil, err
	}
	if platform == "windows" {
		for _, name := range []string{"libgcc_s_seh-1.dll", "libstdc++-6.dll", "libwinpthread-1.dll"} {
			if _, err := os.Stat(filepath.Join(libsDir, name)); err == nil && !contains(names, name) {
				names = append(names, name)
			}
		}
		sort.Strings(names)
	}
	var copied []string
	for _, name := range names {
		source := filepath.Join(libsDir, name)
		destination := filepath.Join(destinationDir, name)
		if err := copyFile(source, destination, 0o755); err != nil {
			warnings = append(warnings, fmt.Sprintf("library %s: %v", name, err))
			continue
		}
		copied = append(copied, name)
	}
	if len(copied) == 0 && len(names) > 0 {
		return nil, warnings, errors.New("no system library could be copied")
	}
	return copied, warnings, nil
}

func isRuntimeLibrary(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".prx") || strings.HasSuffix(lower, ".sprx")
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func resolveClosure(libsDir string, needed []string, neededKnown bool) ([]string, []string, error) {
	var warnings []string
	if !neededKnown {
		entries, err := os.ReadDir(libsDir)
		if err != nil {
			return nil, nil, err
		}
		var all []string
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			lower := strings.ToLower(name)
			if strings.HasSuffix(lower, ".prx") || strings.HasSuffix(lower, ".dll") || strings.HasSuffix(lower, ".so") {
				all = append(all, name)
			}
		}
		sort.Strings(all)
		return all, warnings, nil
	}
	seen := map[string]bool{}
	queue := append([]string(nil), needed...)
	var result []string
	for len(queue) > 0 {
		name := filepath.Base(strings.TrimSpace(queue[0]))
		queue = queue[1:]
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		if _, err := os.Stat(filepath.Join(libsDir, name)); err != nil {
			if isRuntimeLibrary(name) {
				warnings = append(warnings, fmt.Sprintf("needed library %s is not in the AnyPS5 runtime", name))
			}
			continue
		}
		result = append(result, name)
		dependencies, err := importedLibraries(filepath.Join(libsDir, name))
		if err != nil {
			continue
		}
		queue = append(queue, dependencies...)
	}
	sort.Strings(result)
	return result, warnings, nil
}

// bundledModuleDependencies lists the host libraries that the converted
// modules in app0 import. The relinker's needed.json does not always include
// them, so the package resolves them here as well.
func moduleNeededDependencies(outputDir, executable string) []string {
	// The relinker writes <stem>.<module>.needed.json for every converted
	// module. They carry the host libraries that module imports, which the
	// executable's own needed.json does not always include.
	stem := strings.TrimSuffix(filepath.Base(executable), filepath.Ext(executable))
	if stem == "" {
		return nil
	}
	matches, _ := filepath.Glob(filepath.Join(outputDir, stem+".*.needed.json"))
	seen := map[string]bool{}
	var result []string
	for _, path := range matches {
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var names []string
		if json.Unmarshal(body, &names) != nil {
			continue
		}
		for _, name := range names {
			if name != "" && !seen[name] {
				seen[name] = true
				result = append(result, name)
			}
		}
	}
	return result
}

func bundledModuleDependencies(outputDir string) []string {
	seen := map[string]bool{}
	var result []string
	app0 := filepath.Join(outputDir, "app0")
	_ = filepath.WalkDir(app0, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return nil
		}
		name := strings.ToLower(entry.Name())
		if !strings.HasSuffix(name, ".prx") && !strings.HasSuffix(name, ".sprx") {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		dependencies, err := importedLibraries(path)
		if err != nil {
			return nil
		}
		for _, dependency := range dependencies {
			if dependency == "" || seen[dependency] {
				continue
			}
			if strings.Contains(dependency, "$ORIGIN") || strings.Contains(dependency, "/") || strings.Contains(dependency, "\\") {
				continue
			}
			if strings.HasSuffix(strings.ToLower(dependency), ".guest.prx") {
				continue
			}
			seen[dependency] = true
			result = append(result, dependency)
		}
		return nil
	})
	return result
}

func importedLibraries(path string) ([]string, error) {
	if names, err := elfpatch.Needed(path); err == nil {
		return names, nil
	}
	if names, err := peImportedLibraries(path); err == nil {
		return names, nil
	}
	file, err := elf.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return file.ImportedLibraries()
}

func peImportedLibraries(path string) ([]string, error) {
	file, err := pe.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if libraries, err := file.ImportedLibraries(); err == nil && len(libraries) > 0 {
		return libraries, nil
	}
	symbols, err := file.ImportedSymbols()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var libraries []string
	for _, symbol := range symbols {
		if index := strings.LastIndex(symbol, ":"); index >= 0 {
			name := symbol[index+1:]
			if name != "" && !seen[name] {
				seen[name] = true
				libraries = append(libraries, name)
			}
		}
	}
	return libraries, nil
}

func copyResources(source, destination, outputDir string, link bool, excluded map[string]bool) error {
	return copyEntry(source, destination, "", outputDir, link, excluded, true)
}

func copyEntry(source, destination, relative, outputDir string, link bool, excluded map[string]bool, root bool) error {
	relative = filepath.ToSlash(filepath.Clean(relative))
	if !root && excluded[relative] {
		return nil
	}
	if !root && strings.HasPrefix(filepath.Base(source), ".anygame") {
		return nil
	}
	sourceAbsolute, _ := filepath.Abs(source)
	outputAbsolute, _ := filepath.Abs(outputDir)
	if !root && (sourceAbsolute == outputAbsolute || strings.HasPrefix(sourceAbsolute, outputAbsolute+string(os.PathSeparator))) {
		return nil
	}
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(source)
		if err != nil {
			return err
		}
		_ = os.Remove(destination)
		if err := os.Symlink(target, destination); err == nil {
			return nil
		}
		resolved := target
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(filepath.Dir(source), target)
		}
		return copyEntry(resolved, destination, relative, outputDir, link, excluded, false)
	case info.IsDir():
		if err := os.MkdirAll(destination, info.Mode().Perm()|0o700); err != nil {
			return err
		}
		entries, err := os.ReadDir(source)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			childRelative := entry.Name()
			if !root {
				childRelative = filepath.ToSlash(filepath.Join(filepath.FromSlash(relative), entry.Name()))
			}
			if err := copyEntry(filepath.Join(source, entry.Name()), filepath.Join(destination, entry.Name()), childRelative, outputDir, link, excluded, false); err != nil {
				return err
			}
		}
		return nil
	case info.Mode().IsRegular():
		if link {
			_ = os.Remove(destination)
			if err := os.Link(source, destination); err == nil {
				return nil
			}
		}
		return copyFile(source, destination, info.Mode().Perm())
	default:
		return nil
	}
}

func copyFile(source, destination string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func writeLauncher(options Options) (string, error) {
	executable := filepath.Base(options.Executable)
	name := options.Name
	if name == "" {
		name = strings.TrimSuffix(executable, filepath.Ext(executable))
	}
	if options.Target == "windows" {
		path := filepath.Join(options.OutputDir, "run.bat")
		body := "@echo off\r\ncd /d \"%~dp0\"\r\n\"" + executable + "\" %*\r\n"
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return "", err
		}
		return path, nil
	}
	path := filepath.Join(options.OutputDir, "run.sh")
	body := "#!/bin/sh\n# AnyGame launcher for " + name + "\ncd \"$(dirname \"$0\")\" || exit 1\nexec \"./" + executable + "\" \"$@\"\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		return "", err
	}
	return path, nil
}

func writeDesktopEntry(options Options) (string, error) {
	title := options.Name
	if options.Game != nil && options.Game.TitleName != "" {
		title = options.Game.TitleName
	}
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(options.Executable), filepath.Ext(options.Executable))
	}
	icon := "applications-games"
	if options.Game != nil && options.Game.Icon != "" {
		destination := filepath.Join(options.OutputDir, "icon.png")
		if err := copyFile(options.Game.Icon, destination, 0o644); err == nil {
			icon = destination
		}
	}
	executablePath, absErr := filepath.Abs(options.Executable)
	if absErr != nil {
		executablePath = options.Executable
	}
	directory, absErr := filepath.Abs(options.OutputDir)
	if absErr != nil {
		directory = options.OutputDir
	}
	entry := "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=" + sanitizeDesktop(title) + "\n" +
		"Comment=Converted with AnyGame and AnyPS5\n" +
		"Exec=" + quoteDesktop(executablePath) + "\n" +
		"Path=" + directory + "\n" +
		"Icon=" + icon + "\n" +
		"Terminal=false\n" +
		"Categories=Game;\n"
	name := sanitizeFilename(title)
	if name == "" {
		name = "AnyGame"
	}
	path := filepath.Join(options.OutputDir, name+".desktop")
	return path, os.WriteFile(path, []byte(entry), 0o644)
}

func quoteDesktop(value string) string {
	if !strings.ContainsAny(value, " \t\"'\\$`") {
		return value
	}
	return "\"" + strings.NewReplacer("\\", "\\\\", "\"", "\\\"").Replace(value) + "\""
}

func sanitizeDesktop(value string) string {
	return strings.NewReplacer("\n", " ", "\r", " ").Replace(strings.TrimSpace(value))
}

func sanitizeFilename(value string) string {
	var builder strings.Builder
	for _, character := range value {
		switch {
		case character == '/' || character == '\\' || character == ':' || character < 0x20:
			builder.WriteByte('-')
		default:
			builder.WriteRune(character)
		}
	}
	return strings.Trim(builder.String(), " .-")
}
