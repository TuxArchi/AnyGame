// SPDX-License-Identifier: GPL-2.0-only

// Package cli implements the anygame command line.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"anygame/internal/anyps5"
	"anygame/internal/game"
)

// Version is the AnyGame version, overridable at build time.
var Version = "0.1.0-dev"

// Run executes the command line and returns the process exit code.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 1
	}
	command := args[0]
	rest := args[1:]
	switch command {
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	case "version", "--version", "-v":
		return runVersion(rest, stdout, stderr)
	case "update":
		return runUpdate(ctx, rest, stdout, stderr)
	case "runtimes":
		return runRuntimes(rest, stdout, stderr)
	case "info":
		return runInfo(rest, stdout, stderr)
	case "unwrap":
		return runUnwrap(rest, stdout, stderr)
	case "build":
		return runBuild(ctx, rest, stdout, stderr)
	case "run":
		return runGame(ctx, rest, stdout, stderr)
	case "doctor":
		return runDoctor(rest, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "anygame: unknown command %q\n\n", command)
		printUsage(stderr)
		return 1
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `anygame - a product layer for AnyPS5

Usage:
  anygame <command> [options]

Commands:
  update      Download or refresh AnyPS5 release binaries and libraries
  runtimes    List the installed AnyPS5 runtimes
  info        Inspect an extracted PS5 title folder
  build       Convert a title and assemble a runnable package
  run         Launch a converted package
  unwrap      Unwrap a decrypted SELF container into a clean ELF
  doctor      Check host prerequisites
  version     Print the AnyGame version

Environment:
  ANYGAME_HOME   Data directory (default: per-user data directory)
  GITHUB_TOKEN   Raises the GitHub API rate limit

Run "anygame <command> --help" for the options of a command.
`)
}

func runVersion(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("version", flag.ContinueOnError)
	flags.SetOutput(stderr)
	jsonOut := flags.Bool("json", false, "print JSON")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	home, _ := anyps5.Home()
	info := map[string]any{
		"version":    Version,
		"go":         runtime.Version(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"anyps5Home": home,
	}
	if *jsonOut {
		writeJSON(stdout, info)
		return 0
	}
	fmt.Fprintf(stdout, "anygame %s (%s/%s, %s)\n", Version, runtime.GOOS, runtime.GOARCH, runtime.Version())
	fmt.Fprintf(stdout, "data: %s\n", home)
	return 0
}

func writeJSON(w io.Writer, value any) {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Fprintf(w, "{\"error\":%q}\n", err.Error())
		return
	}
	fmt.Fprintln(w, string(body))
}

func fail(stderr io.Writer, err error) int {
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	fmt.Fprintf(stderr, "error: %v\n", err)
	return 1
}

func requirePositional(flags *flag.FlagSet, name string) (string, error) {
	if flags.NArg() < 1 {
		return "", fmt.Errorf("missing %s", name)
	}
	return flags.Arg(0), nil
}

// parsePositionals parses flags that may follow positional arguments, which
// the standard flag package does not accept. boolFlags names the flags that
// take no value.
func parsePositionals(flags *flag.FlagSet, args []string, boolFlags ...string) ([]string, error) {
	bools := map[string]bool{}
	for _, name := range boolFlags {
		bools[name] = true
	}
	var flagArgs, positional []string
	for index := 0; index < len(args); index++ {
		argument := args[index]
		if argument == "--" {
			positional = append(positional, args[index+1:]...)
			break
		}
		if len(argument) > 1 && argument[0] == '-' {
			name := strings.TrimLeft(argument, "-")
			if equals := strings.IndexByte(name, '='); equals >= 0 {
				name = name[:equals]
			}
			flagArgs = append(flagArgs, argument)
			if !bools[name] && index+1 < len(args) && !strings.HasPrefix(args[index+1], "-") {
				flagArgs = append(flagArgs, args[index+1])
				index++
			}
			continue
		}
		positional = append(positional, argument)
	}
	if err := flags.Parse(flagArgs); err != nil {
		return nil, err
	}
	return positional, nil
}

func outputPath(value, fallback string) (string, error) {
	if value == "" {
		value = fallback
	}
	return filepath.Abs(value)
}

func samePath(a, b string) bool {
	absoluteA, errA := filepath.Abs(a)
	absoluteB, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return false
	}
	if filepath.Clean(absoluteA) == filepath.Clean(absoluteB) {
		return true
	}
	infoA, errA := os.Stat(absoluteA)
	infoB, errB := os.Stat(absoluteB)
	return errA == nil && errB == nil && os.SameFile(infoA, infoB)
}

func sanitizeFilename(value string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	for _, character := range value {
		switch {
		case character == '/' || character == '\\' || character == ':' || character < 0x20:
			builder.WriteByte('-')
		default:
			builder.WriteRune(character)
		}
	}
	result := strings.Trim(builder.String(), " .-")
	if result == "" {
		return "game"
	}
	return result
}

// StringList collects a repeatable flag.
type StringList []string

func (l *StringList) String() string { return strings.Join(*l, ",") }

func (l *StringList) Set(value string) error {
	*l = append(*l, value)
	return nil
}

// resolveTitleName picks a stable executable base name.
func resolveTitleName(info *game.Info) string {
	if info != nil && info.TitleID != "" {
		return sanitizeFilename(info.TitleID)
	}
	if info != nil && info.TitleName != "" {
		return sanitizeFilename(info.TitleName)
	}
	return "game"
}
