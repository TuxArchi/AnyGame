// SPDX-License-Identifier: GPL-2.0-only

package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"anygame/internal/anyps5"
	"anygame/internal/game"
	"anygame/internal/self"
)

func runUpdate(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("update", flag.ContinueOnError)
	flags.SetOutput(stderr)
	version := flags.String("version", "", "AnyPS5 release to install (default: latest)")
	target := flags.String("target", runtime.GOOS, "target platform: linux, windows or all")
	force := flags.Bool("force", false, "reinstall even when the runtime is present")
	check := flags.Bool("check", false, "report whether an update is available")
	list := flags.Bool("list", false, "list recent AnyPS5 releases")
	jsonOut := flags.Bool("json", false, "print JSON")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		return fail(stderr, fmt.Errorf("update takes no positional arguments"))
	}
	client := anyps5.NewClient()

	if *list {
		releases, err := client.List(ctx, 20)
		if err != nil {
			return fail(stderr, err)
		}
		if *jsonOut {
			writeJSON(stdout, releases)
			return 0
		}
		for _, release := range releases {
			fmt.Fprintf(stdout, "%-12s %s\n", release.Tag, release.PublishedAt.Format("2006-01-02"))
		}
		return 0
	}

	targets, err := expandTargets(*target)
	if err != nil {
		return fail(stderr, err)
	}
	if *check {
		type checkResult struct {
			Target    string `json:"target"`
			Latest    string `json:"latest"`
			Installed string `json:"installed"`
			Update    bool   `json:"updateAvailable"`
		}
		var results []checkResult
		for _, platform := range targets {
			release, err := client.Latest(ctx)
			if err != nil {
				return fail(stderr, err)
			}
			active := anyps5.ActiveVersion(platform)
			item := checkResult{Target: platform, Latest: release.Version(), Installed: active}
			item.Update = active != release.Version()
			results = append(results, item)
		}
		if *jsonOut {
			writeJSON(stdout, results)
			return 0
		}
		for _, item := range results {
			if item.Update {
				if item.Installed == "" {
					fmt.Fprintf(stdout, "%s: AnyPS5 %s available (not installed)\n", item.Target, item.Latest)
				} else {
					fmt.Fprintf(stdout, "%s: AnyPS5 %s available (installed: %s)\n", item.Target, item.Latest, item.Installed)
				}
			} else {
				fmt.Fprintf(stdout, "%s: up to date (AnyPS5 %s)\n", item.Target, item.Latest)
			}
		}
		return 0
	}

	type updateResult struct {
		Target           string `json:"target"`
		Version          string `json:"version"`
		Directory        string `json:"directory"`
		AlreadyInstalled bool   `json:"alreadyInstalled"`
	}
	var results []updateResult
	for _, platform := range targets {
		fmt.Fprintf(stderr, "AnyPS5 %s: resolving release...\n", platform)
		progress := newProgress(stderr)
		installed, err := anyps5.Install(ctx, client, platform, *version, *force, progress.update)
		progress.done()
		if err != nil {
			return fail(stderr, err)
		}
		results = append(results, updateResult{
			Target: platform, Version: installed.Version, Directory: installed.Dir,
			AlreadyInstalled: installed.AlreadyInstalled,
		})
		if !*jsonOut {
			if installed.AlreadyInstalled {
				fmt.Fprintf(stdout, "AnyPS5 %s for %s is already installed at %s\n", installed.Version, platform, installed.Dir)
			} else {
				fmt.Fprintf(stdout, "Installed AnyPS5 %s for %s at %s\n", installed.Version, platform, installed.Dir)
			}
		}
	}
	if *jsonOut {
		writeJSON(stdout, results)
	}
	return 0
}

func expandTargets(value string) ([]string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "host":
		return []string{runtime.GOOS}, nil
	case "all":
		return []string{"linux", "windows"}, nil
	default:
		target, err := anyps5.NormalizeTarget(value)
		if err != nil {
			return nil, err
		}
		return []string{target}, nil
	}
}

func runRuntimes(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("runtimes", flag.ContinueOnError)
	flags.SetOutput(stderr)
	jsonOut := flags.Bool("json", false, "print JSON")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	runtimes, err := anyps5.List(true)
	if err != nil {
		return fail(stderr, err)
	}
	if *jsonOut {
		writeJSON(stdout, runtimes)
		return 0
	}
	if len(runtimes) == 0 {
		home, _ := anyps5.Home()
		fmt.Fprintf(stdout, "No AnyPS5 runtimes installed under %s\nRun `anygame update` to install one.\n", home)
		return 0
	}
	for _, item := range runtimes {
		active := ""
		if anyps5.ActiveVersion(item.Target) == item.Version {
			active = " (active)"
		}
		fmt.Fprintf(stdout, "%s  %-8s %s%s\n  %s\n", item.Version, item.Target, item.Manifest.InstalledAt.Format("2006-01-02"), active, item.Dir)
	}
	return 0
}

type progressPrinter struct {
	writer      io.Writer
	interactive bool
	active      bool
	last        time.Time
	lastDone    int64
	lastTotal   int64
}

func newProgress(writer io.Writer) *progressPrinter {
	interactive := false
	if file, ok := writer.(*os.File); ok {
		if info, err := file.Stat(); err == nil {
			interactive = info.Mode()&os.ModeCharDevice != 0
		}
	}
	return &progressPrinter{writer: writer, interactive: interactive}
}

func (p *progressPrinter) update(done, total int64) {
	p.lastDone, p.lastTotal = done, total
	if !p.interactive {
		return
	}
	now := time.Now()
	if now.Sub(p.last) < 150*time.Millisecond && done < total {
		return
	}
	p.last = now
	if total > 0 {
		percent := float64(done) * 100 / float64(total)
		fmt.Fprintf(p.writer, "\r  downloading %5.1f%%  %s / %s", percent, humanBytes(done), humanBytes(total))
	} else {
		fmt.Fprintf(p.writer, "\r  downloading %s", humanBytes(done))
	}
	p.active = true
}

func (p *progressPrinter) done() {
	if p.interactive {
		if p.active {
			fmt.Fprint(p.writer, "\r\x1b[K")
			p.active = false
		}
		return
	}
	if p.lastDone > 0 {
		if p.lastTotal > 0 {
			fmt.Fprintf(p.writer, "  downloaded %s / %s\n", humanBytes(p.lastDone), humanBytes(p.lastTotal))
		} else {
			fmt.Fprintf(p.writer, "  downloaded %s\n", humanBytes(p.lastDone))
		}
	}
}

func humanBytes(value int64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	divisor, exponent := int64(unit), 0
	for value/divisor >= unit && exponent < 5 {
		divisor *= unit
		exponent++
	}
	return fmt.Sprintf("%.1f %ciB", float64(value)/float64(divisor), "KMGTPE"[exponent])
}

func runInfo(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("info", flag.ContinueOnError)
	flags.SetOutput(stderr)
	jsonOut := flags.Bool("json", false, "print JSON")
	positionals, err := parsePositionals(flags, args, "json")
	if err != nil {
		return 2
	}
	if len(positionals) != 1 {
		return fail(stderr, fmt.Errorf("usage: anygame info <title path> [--json]"))
	}
	path := positionals[0]
	info, err := game.Discover(path)
	if err != nil {
		return fail(stderr, err)
	}
	if *jsonOut {
		writeJSON(stdout, info)
		return 0
	}
	fmt.Fprintf(stdout, "Title:       %s\n", valueOr(info.TitleName, "(unknown)"))
	fmt.Fprintf(stdout, "Title ID:    %s\n", valueOr(info.TitleID, "(unknown)"))
	fmt.Fprintf(stdout, "Version:     %s\n", valueOr(info.ContentVersion, "(unknown)"))
	fmt.Fprintf(stdout, "Language:    %s\n", valueOr(info.Language, "(default)"))
	fmt.Fprintf(stdout, "Executable:  %s (%s, %s)\n", info.Eboot, info.Kind, humanBytes(info.EbootSize))
	fmt.Fprintf(stdout, "Resources:   %s\n", info.Resources)
	if len(info.ModuleDirs) == 0 {
		fmt.Fprintln(stdout, "Modules:     none")
	} else {
		fmt.Fprintf(stdout, "Modules:     %s\n", strings.Join(info.ModuleDirs, ", "))
	}
	if info.ParamJSON != "" {
		fmt.Fprintf(stdout, "Metadata:    %s\n", info.ParamJSON)
	} else if info.ParamSFO != "" {
		fmt.Fprintf(stdout, "Metadata:    %s\n", info.ParamSFO)
	}
	if info.Icon != "" {
		fmt.Fprintf(stdout, "Icon:        %s\n", info.Icon)
	}
	for _, warning := range info.Warnings {
		fmt.Fprintf(stdout, "warning:     %s\n", warning)
	}
	return 0
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func runUnwrap(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("unwrap", flag.ContinueOnError)
	flags.SetOutput(stderr)
	verbose := flags.Bool("verbose", false, "print the program header mapping")
	positionals, err := parsePositionals(flags, args, "verbose")
	if err != nil {
		return 2
	}
	if len(positionals) != 2 {
		return fail(stderr, fmt.Errorf("usage: anygame unwrap <input.self> <output.elf> [--verbose]"))
	}
	var verboseWriter io.Writer
	if *verbose {
		verboseWriter = stderr
	}
	result, err := self.UnwrapFile(positionals[0], positionals[1], verboseWriter)
	if err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintf(stdout, "wrote %s, %d bytes\n", positionals[1], result.OutputBytes)
	return 0
}

func defaultOutputDir(info *game.Info) string {
	name := resolveTitleName(info)
	working, err := filepath.Abs(".")
	if err != nil {
		return name
	}
	return filepath.Join(working, name)
}
